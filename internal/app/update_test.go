package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"yups/internal/config"
	"yups/internal/update"
)

// Repository URLs matching config.Defaults(), plus their derived API
// endpoints, so canned responses can be keyed by exact URL.
const (
	testRepoPrimary   = "https://code.javilopezg.com/javilopezg/yups"
	testRepoFallback  = "https://github.com/JaviLopezG/yups"
	testPrimaryAPI    = "https://code.javilopezg.com/api/v1/repos/javilopezg/yups/releases/latest"
	testFallbackAPI   = "https://api.github.com/repos/JaviLopezG/yups/releases/latest"
	testArchiveURL    = "https://dl.example/yups.tar.gz"
	testChecksumsURL  = "https://dl.example/checksums.txt"
	testStagingBinary = "/tmp/stage-abc123/yups"
)

// fakeTransport serves canned HTTP responses without touching the network;
// every request URL is recorded so tests can assert that no call happened.
type fakeTransport struct {
	responses map[string]*http.Response
	hits      *[]string
}

func (t *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	*t.hits = append(*t.hits, req.URL.String())
	resp, ok := t.responses[req.URL.String()]
	if !ok {
		return nil, fmt.Errorf("no fake response for %s", req.URL)
	}
	return resp, nil
}

func fakeResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// setVersion overrides the ldflags-injected package version for a test.
func setVersion(t *testing.T, v string) {
	t.Helper()
	old := Version
	Version = v
	t.Cleanup(func() { Version = old })
}

// releaseDocument builds a releases/latest JSON body whose asset URLs are
// the fixed test endpoints.
func releaseDocument(tag string) string {
	return fmt.Sprintf(`{"tag_name": %q, "assets": [
		{"name": "yups_%s_linux_amd64.tar.gz", "browser_download_url": %q},
		{"name": %q, "browser_download_url": %q}]}`,
		tag, tag, testArchiveURL, update.ChecksumsFileName, testChecksumsURL)
}

func sha256Hex(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

// serveLatestRelease registers valid release documents (with a matching
// checksums.txt for the given archive payload) on both the primary and
// the fallback endpoint.
func serveLatestRelease(fs *fakeFS, tag, archivePayload string) {
	document := releaseDocument(tag)
	fs.httpResponses[testPrimaryAPI] = fakeResponse(http.StatusOK, document)
	fs.httpResponses[testFallbackAPI] = fakeResponse(http.StatusOK, document)
	fs.httpResponses[testArchiveURL] = fakeResponse(http.StatusOK, archivePayload)
	checksums := fmt.Sprintf("%s  %s\n", sha256Hex(archivePayload), "yups_"+tag+"_linux_amd64.tar.gz")
	fs.httpResponses[testChecksumsURL] = fakeResponse(http.StatusOK, checksums)
}

// storeConfig places a configuration file in the fake home.
func storeConfig(fs *fakeFS, c config.Config) {
	fs.configs[config.Path(fs.home)] = c
}

func TestUpdateUpToDateExitsWithoutTouchingSystem(t *testing.T) {
	setVersion(t, "v1.0.0")
	fs := newFakeFS()
	serveLatestRelease(fs, "v1.0.0", "payload")

	out, code := runDispatch(t, fs.env(), "--update-yups")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	if !strings.Contains(out, "up to date") {
		t.Errorf("output %q does not report being up to date", out)
	}
	if len(fs.execCalls) != 0 {
		t.Errorf("nothing should be executed when up to date: %+v", fs.execCalls)
	}
	if len(fs.replacedDirs) != 0 {
		t.Errorf("system was touched while up to date: %v", fs.replacedDirs)
	}
}

func TestUpdateCorruptConfigAbortsBeforeAnyNetworkCall(t *testing.T) {
	setVersion(t, "v1.0.0")
	fs := newFakeFS()
	fs.corruptConfig[config.Path(fs.home)] = true

	out, code := runDispatch(t, fs.env(), "--update-yups")
	if code != ExitError {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitError, out)
	}
	if !strings.Contains(out, config.Path(fs.home)) {
		t.Errorf("error %q does not mention the offending file", out)
	}
	if len(fs.httpHits) != 0 {
		t.Errorf("network was used despite the corrupt config: %v", fs.httpHits)
	}
}

func TestUpdateFallsBackWhenPrimarySourceFails(t *testing.T) {
	setVersion(t, "v1.0.0")
	fs := newFakeFS()
	fs.httpResponses[testPrimaryAPI] = fakeResponse(http.StatusInternalServerError, "boom")
	fs.httpResponses[testFallbackAPI] = fakeResponse(http.StatusOK, releaseDocument("v1.1.0"))
	fs.httpResponses[testArchiveURL] = fakeResponse(http.StatusOK, "payload")
	fs.httpResponses[testChecksumsURL] = fakeResponse(http.StatusOK,
		fmt.Sprintf("%s  yups_v1.1.0_linux_amd64.tar.gz\n", sha256Hex("payload")))
	fs.stagedBinary = testStagingBinary
	fs.addExecutable("/usr/bin/yups")

	out, code := runDispatch(t, fs.env(), "--update-yups")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	if !strings.Contains(out, "fallback") {
		t.Errorf("output %q does not mention the fallback source", out)
	}
	if len(fs.execCalls) != 1 {
		t.Fatalf("expected exactly one ExecSelf call, got %+v", fs.execCalls)
	}
}

func TestUpdateBothSourcesDownFails(t *testing.T) {
	setVersion(t, "v1.0.0")
	fs := newFakeFS()
	fs.httpResponses[testPrimaryAPI] = fakeResponse(http.StatusServiceUnavailable, "down")
	fs.httpResponses[testFallbackAPI] = fakeResponse(http.StatusServiceUnavailable, "down")

	out, code := runDispatch(t, fs.env(), "--update-yups")
	if code != ExitError {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitError, out)
	}
	if !strings.Contains(out, "fallback failed too") {
		t.Errorf("output %q does not report both sources failing", out)
	}
	if len(fs.execCalls) != 0 {
		t.Errorf("nothing should be executed when no source answers: %+v", fs.execCalls)
	}
}

func TestUpdateBadChecksumAbortsUntouched(t *testing.T) {
	setVersion(t, "v1.0.0")
	fs := newFakeFS()
	document := releaseDocument("v1.1.0")
	fs.httpResponses[testPrimaryAPI] = fakeResponse(http.StatusOK, document)
	fs.httpResponses[testArchiveURL] = fakeResponse(http.StatusOK, "tampered payload")
	fs.httpResponses[testChecksumsURL] = fakeResponse(http.StatusOK,
		fmt.Sprintf("%s  yups_v1.1.0_linux_amd64.tar.gz\n", sha256Hex("original payload")))
	fs.stagedBinary = testStagingBinary
	fs.addExecutable("/usr/bin/yups")

	out, code := runDispatch(t, fs.env(), "--update-yups")
	if code != ExitError {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitError, out)
	}
	if !strings.Contains(out, "checksum mismatch") {
		t.Errorf("output %q does not report the checksum failure", out)
	}
	if len(fs.execCalls) != 0 || len(fs.replacedDirs) != 0 {
		t.Errorf("system was touched despite the bad checksum: %+v %v", fs.execCalls, fs.replacedDirs)
	}
}

func TestUpdateExecsApplierWithHandoffArguments(t *testing.T) {
	setVersion(t, "v1.0.0")
	fs := newFakeFS()
	serveLatestRelease(fs, "v1.1.0", "payload")
	fs.stagedBinary = testStagingBinary
	fs.addExecutable("/usr/bin/yups")

	out, code := runDispatch(t, fs.env(), "--update-yups")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	wantArgv := []string{ProgramName, flagUpdateApply, "--from", "/tmp/stage-abc123", "--installed", "/usr/bin"}
	call := fs.execCalls[0]
	if call.path != testStagingBinary {
		t.Errorf("exec path = %q, want the staged binary %q", call.path, testStagingBinary)
	}
	if strings.Join(call.argv, " ") != strings.Join(wantArgv, " ") {
		t.Errorf("exec argv = %v, want %v", call.argv, wantArgv)
	}
}

func TestUpdateWithoutInstalledCopyInformsAndCleansStaging(t *testing.T) {
	setVersion(t, "v1.0.0")
	fs := newFakeFS()
	serveLatestRelease(fs, "v1.1.0", "payload")
	fs.stagedBinary = testStagingBinary // StageBinary succeeded...

	out, code := runDispatch(t, fs.env(), "--update-yups") // ...but nothing is installed
	if code != ExitError {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitError, out)
	}
	if !strings.Contains(out, "--install") {
		t.Errorf("output %q does not suggest installing instead", out)
	}
	if !containsString(fs.removedAll, "/tmp/stage-abc123") {
		t.Errorf("staging directory was not cleaned: %v", fs.removedAll)
	}
	if len(fs.execCalls) != 0 {
		t.Errorf("nothing should be executed without an installation: %+v", fs.execCalls)
	}
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func TestUpdateApplyOperatesOnKeeperAndInformsDuplicates(t *testing.T) {
	setVersion(t, "v1.1.0")
	fs := newFakeFS()
	fs.addExecutable("/usr/bin/yups")

	out, code := runDispatch(t, fs.env(),
		flagUpdateApply, "--from", "/tmp/stage-abc123", "--installed", "/usr/bin,/home/user/bin")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	// Decision 8: only the keeper is replaced; duplicates survive.
	if len(fs.replacedDirs) != 1 || fs.replacedDirs[0] != "/usr/bin" {
		t.Errorf("replaced = %v, want only the keeper /usr/bin", fs.replacedDirs)
	}
	requireOutputContains(t, out,
		"Updated /usr/bin/"+ProgramName,
		"several places",
		"/home/user/bin/"+ProgramName,
		"duplicates are left untouched")
	if containsString(fs.replacedDirs, "/home/user/bin") {
		t.Error("the duplicate must not be touched")
	}
	if !containsString(fs.removedAll, "/tmp/stage-abc123") {
		t.Errorf("staging directory was not cleaned: %v", fs.removedAll)
	}
	if !strings.Contains(out, "updated to v1.1.0") {
		t.Errorf("output %q does not announce the new version", out)
	}
}

func TestSelectKeeperPriorityRules(t *testing.T) {
	tests := []struct {
		name     string
		dirs     []string
		running  string
		want     string
		wantRest string // comma joined expected others
	}{
		{
			name:     "first result wins without signals",
			dirs:     []string{"/usr/bin", "/home/user/bin"},
			running:  "",
			want:     "/usr/bin",
			wantRest: "/home/user/bin",
		},
		{
			name:     "directory outside usr and home wins",
			dirs:     []string{"/home/user/bin", "/opt/bin"},
			want:     "/opt/bin",
			wantRest: "/home/user/bin",
		},
		{
			name:     "running executable wins among ties",
			dirs:     []string{"/opt/a", "/opt/b"},
			running:  "/opt/b/yups",
			want:     "/opt/b",
			wantRest: "/opt/a",
		},
		{
			name:     "outside beats the running executable in system dirs",
			dirs:     []string{"/usr/bin", "/opt/bin", "/home/user/bin"},
			running:  "/usr/bin/yups",
			want:     "/opt/bin",
			wantRest: "/usr/bin,/home/user/bin",
		},
		{
			name: "duplicates collapse to one candidate",
			dirs: []string{"/usr/bin", "/usr/bin"},
			want: "/usr/bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keeper, others := selectKeeper(tt.dirs, tt.running)
			if keeper != tt.want {
				t.Errorf("keeper = %q, want %q", keeper, tt.want)
			}
			if got := strings.Join(others, ","); got != tt.wantRest {
				t.Errorf("others = %q, want %q", got, tt.wantRest)
			}
		})
	}
}

func TestUpdateApplyBumpsConfigForwardNeverBackwards(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		want       string
	}{
		{"older config advances", "v0.9.0", "v1.1.0"},
		{"newer config is kept", "v2.0.0", "v2.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setVersion(t, "v1.1.0")
			fs := newFakeFS()
			storeConfig(fs, config.Config{
				Version:          tt.configured,
				YUPSRepo:         testRepoPrimary,
				YUPSRepoFallback: testRepoFallback,
			})

			out, code := runDispatch(t, fs.env(),
				flagUpdateApply, "--from", "/tmp/stage-abc123", "--installed", "/usr/bin")
			if code != ExitOK {
				t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
			}
			got := fs.configs[config.Path(fs.home)].Version
			if got != tt.want {
				t.Errorf("config.version = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUpdateApplyKeepsStagingWhenBlockedAndSuggestsSudo(t *testing.T) {
	setVersion(t, "v1.1.0")
	fs := newFakeFS()
	fs.groups = []string{"sudo"}
	fs.replaceErrs["/usr/bin"] = errors.New("permission denied")

	out, code := runDispatch(t, fs.env(),
		flagUpdateApply, "--from", "/tmp/stage-abc123", "--installed", "/usr/bin")
	if code != ExitError {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitError, out)
	}
	if !strings.Contains(out, "sudo !!") {
		t.Errorf("output %q does not suggest the sudo retry", out)
	}
	if containsString(fs.removedAll, "/tmp/stage-abc123") {
		t.Error("staging was cleaned although a retry with sudo still needs it")
	}
}

func TestUpdateApplyUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"missing from", []string{}},
		{"from without value", []string{"--from"}},
		{"unknown option", []string{"--from", "/tmp/x", "--bogus"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{flagUpdateApply}, tt.args...)
			out, code := runDispatch(t, newFakeFS().env(), args...)
			if code != ExitUsage {
				t.Fatalf("exit code = %d, want %d (%s)", code, ExitUsage, out)
			}
		})
	}
}

func TestRunMigrationsAppliesPendingInOrderAndRecordsProgress(t *testing.T) {
	restoreMigrations(t,
		stepMigration("v1.1.0", &ranSteps),
		stepMigration("v1.2.0", &ranSteps),
	)
	ranSteps = nil

	fs := newFakeFS()
	applied, err := RunMigrations(fs.env(), fs.home, "v1.2.0")
	if err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	if applied != 2 {
		t.Errorf("applied = %d, want 2", applied)
	}
	if strings.Join(ranSteps, ",") != "v1.1.0,v1.2.0" {
		t.Errorf("ran = %v, want ascending order [v1.1.0 v1.2.0]", ranSteps)
	}
	if got := fs.stateLast[statePath(fs.home)]; got != "v1.2.0" {
		t.Errorf("recorded progress = %q, want v1.2.0", got)
	}
}

var ranSteps []string

func stepMigration(version string, ran *[]string) Migration {
	return Migration{
		Version: version,
		Name:    "step-" + version,
		Apply: func(env *Env, home string) error {
			*ran = append(*ran, version)
			return nil
		},
	}
}

// restoreMigrations swaps the global migration registry and restores it
// when the test finishes.
func restoreMigrations(t *testing.T, steps ...Migration) {
	t.Helper()
	old := migrations
	migrations = steps
	t.Cleanup(func() { migrations = old })
}

func TestRunMigrationsSkipsAppliedAndOutOfRangeSteps(t *testing.T) {
	restoreMigrations(t,
		stepMigration("v1.1.0", &ranSteps),
		stepMigration("v1.2.0", &ranSteps),
		stepMigration("v1.3.0", &ranSteps),
	)
	ranSteps = nil

	fs := newFakeFS()
	fs.stateLast[statePath(fs.home)] = "v1.1.0" // v1.1.0 already applied

	applied, err := RunMigrations(fs.env(), fs.home, "v1.2.0")
	if err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	if applied != 1 || strings.Join(ranSteps, ",") != "v1.2.0" {
		t.Errorf("applied = %d, ran = %v; want only v1.2.0", applied, ranSteps)
	}
}

func TestRunMigrationsDevTargetAppliesNothing(t *testing.T) {
	restoreMigrations(t, stepMigration("v1.1.0", &ranSteps))
	ranSteps = nil

	fs := newFakeFS()
	applied, err := RunMigrations(fs.env(), fs.home, "dev")
	if err != nil || applied != 0 || len(ranSteps) != 0 {
		t.Errorf("applied = %d, ran = %v, err = %v; want nothing to run for a dev build", applied, ranSteps, err)
	}
}

func TestRunMigrationsFailureStopsAndKeepsPreviousProgress(t *testing.T) {
	restoreMigrations(t,
		stepMigration("v1.1.0", &ranSteps),
		Migration{
			Version: "v1.2.0",
			Name:    "broken step",
			Apply: func(env *Env, home string) error {
				return errors.New("boom")
			},
		},
	)
	ranSteps = nil

	fs := newFakeFS()
	applied, err := RunMigrations(fs.env(), fs.home, "v1.2.0")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("RunMigrations error = %v, want the step failure", err)
	}
	if applied != 1 {
		t.Errorf("applied = %d, want 1 (only the step before the failure)", applied)
	}
	if got := fs.stateLast[statePath(fs.home)]; got != "v1.1.0" {
		t.Errorf("recorded progress = %q, want v1.1.0", got)
	}
}

func TestMigrationPendingBoundaries(t *testing.T) {
	tests := []struct {
		name        string
		lastApplied string
		version     string
		target      string
		want        bool
	}{
		{"between last and target runs", "v1.0.0", "v1.1.0", "v1.2.0", true},
		{"already applied skips", "v1.1.0", "v1.1.0", "v1.2.0", false},
		{"newer than target skips", "v1.0.0", "v1.3.0", "v1.2.0", false},
		{"empty history runs everything", "", "v1.1.0", "v1.2.0", true},
		{"equal to target runs", "v1.0.0", "v1.2.0", "v1.2.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := migrationPending(tt.lastApplied, tt.version, tt.target); got != tt.want {
				t.Errorf("migrationPending(%q, %q, %q) = %v, want %v",
					tt.lastApplied, tt.version, tt.target, got, tt.want)
			}
		})
	}
}

// requireOutputContains asserts every fragment appears in out.
func requireOutputContains(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q:\n%s", want, out)
		}
	}
}

func TestInstallReportsMultiLocationAnomaly(t *testing.T) {
	fs := newFakeFS()
	fs.addExecutable("/home/user/bin/yups") // first PATH directory
	fs.addExecutable("/usr/bin/yups")

	out, code := runDispatch(t, fs.env(), "--install")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	requireOutputContains(t, out,
		"already installed",
		"several places",
		"consider cleaning up the duplicates",
		"will keep using /home/user/bin/"+ProgramName)
	if fs.installedDst != "" {
		t.Error("nothing should have been copied over an existing installation")
	}
}

func TestUninstallSeveralCopiesAsksForAllUsers(t *testing.T) {
	tests := []struct {
		name         string
		files        []string
		answer       bool
		wantRemoved  []string
		wantKept     []string
		wantExitCode int
	}{
		{
			name:         "declining keeps the system copies",
			files:        []string{"/home/user/bin/yups", "/usr/bin/yups"},
			answer:       false,
			wantRemoved:  []string{"/home/user/bin/yups"},
			wantKept:     []string{"/usr/bin/yups"},
			wantExitCode: ExitOK,
		},
		{
			name:         "declining without home copies removes nothing",
			files:        []string{"/usr/bin/yups", "/usr/local/bin/yups"},
			answer:       false,
			wantRemoved:  nil,
			wantKept:     []string{"/usr/bin/yups", "/usr/local/bin/yups"},
			wantExitCode: ExitOK,
		},
		{
			name:         "accepting removes everything possible",
			files:        []string{"/home/user/bin/yups", "/usr/bin/yups"},
			answer:       true,
			wantRemoved:  []string{"/home/user/bin/yups", "/usr/bin/yups"},
			wantKept:     nil,
			wantExitCode: ExitOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFakeFS()
			for _, file := range tt.files {
				fs.addExecutable(file)
			}
			fs.askScript = []bool{tt.answer}

			out, code := runDispatch(t, fs.env(), "--uninstall")
			if code != tt.wantExitCode {
				t.Fatalf("exit code = %d, want %d (%s)", code, tt.wantExitCode, out)
			}
			if len(fs.askedQuestions) != 1 || !strings.Contains(fs.askedQuestions[0], "all users") {
				t.Errorf("asked = %v, want exactly the all-users question", fs.askedQuestions)
			}
			for _, path := range tt.wantRemoved {
				if !containsString(fs.removed, path) {
					t.Errorf("%s was not removed (removed: %v)", path, fs.removed)
				}
				if !strings.Contains(out, "Removed "+path) {
					t.Errorf("output %q does not report removing %s", out, path)
				}
			}
			for _, path := range tt.wantKept {
				if !fs.files[path] {
					t.Errorf("%s should have survived", path)
				}
			}
		})
	}
}

func TestUninstallAsksToDeleteStateDir(t *testing.T) {
	stateDir := config.Dir("/home/user")

	t.Run("default keeps the state directory", func(t *testing.T) {
		fs := newFakeFS()
		fs.addExecutable("/usr/bin/yups")
		fs.existingPaths[stateDir] = true

		out, code := runDispatch(t, fs.env(), "--uninstall")
		if code != ExitOK {
			t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
		}
		requireOutputContains(t, out, "Keeping "+stateDir)
		if containsString(fs.removedAll, stateDir) {
			t.Error("the state directory must survive by default")
		}
	})

	t.Run("confirming deletes the state directory", func(t *testing.T) {
		fs := newFakeFS()
		fs.addExecutable("/usr/bin/yups")
		fs.existingPaths[stateDir] = true
		fs.askScript = []bool{true}

		out, code := runDispatch(t, fs.env(), "--uninstall")
		if code != ExitOK {
			t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
		}
		requireOutputContains(t, out, "Deleted "+stateDir)
		if !containsString(fs.removedAll, stateDir) {
			t.Errorf("the state directory was not deleted: %v", fs.removedAll)
		}
	})

	t.Run("no question without a state directory", func(t *testing.T) {
		fs := newFakeFS()
		fs.addExecutable("/usr/bin/yups")

		out, code := runDispatch(t, fs.env(), "--uninstall")
		if code != ExitOK {
			t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
		}
		if len(fs.askedQuestions) != 0 {
			t.Errorf("unexpected questions: %v", fs.askedQuestions)
		}
	})
}

func TestUpdateWithoutStateDirWarnsAndOffersPerUserInstall(t *testing.T) {
	setVersion(t, "v1.0.0")
	fs := newFakeFS()
	serveLatestRelease(fs, "v1.0.0", "payload")
	fs.writable["/usr/local/bin"] = true // so the offered install can succeed
	fs.askScript = []bool{true}          // accept the per-user install

	out, code := runDispatch(t, fs.env(), "--update-yups")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	requireOutputContains(t, out,
		"is not initialized",
		"limited functionality",
		"up to date")
	if fs.installedDst != "/usr/local/bin/yups" {
		t.Errorf("per-user install did not run (installed to %q)", fs.installedDst)
	}
	if len(fs.askedQuestions) != 1 || !strings.Contains(fs.askedQuestions[0], "per-user installation") {
		t.Errorf("asked = %v, want exactly the per-user install offer", fs.askedQuestions)
	}
}
