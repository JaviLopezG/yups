//go:build integration

// Integration tests: they build the yups binary for linux and exercise it
// inside stock distro containers (ubuntu, fedora, archlinux and opensuse;
// see allDistros below) with different users and permissions.
//
// Run them with:
//
//	make test-integration   (or: go test -tags integration ./integration/)
package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// defaultPATH is the PATH of a stock container image (every distro below
// ships this value).
const defaultPATH = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// distro describes one of the systems the integration suite can run on.
type distro struct {
	name       string // short selector name used in YUPS_TEST_DISTRO
	image      string // docker hub reference
	adminGroup string // administrator group the distro ships by default
	setup      string // optional root script executed right after starting the container
}

// allDistros lists every system the suite knows about.
var allDistros = []distro{
	{name: "ubuntu", image: "ubuntu:latest", adminGroup: "sudo"},
	{name: "fedora", image: "fedora:latest", adminGroup: "wheel"},
	// The arch image may not ship useradd/groupadd (the shadow
	// package); install it when missing.
	{name: "arch", image: "archlinux:latest", adminGroup: "wheel", setup: "command -v useradd >/dev/null 2>&1 || pacman -Sy --noconfirm shadow"},
	// openSUSE only creates the wheel group when the installer (YaST)
	// grants administrator rights to a user; a stock container lacks it,
	// so reproduce that configuration step.
	{name: "opensuse", image: "opensuse/leap:latest", adminGroup: "wheel", setup: "groupadd -f wheel"},
}

// distroEnvVar selects which distros the suite runs against: a comma
// separated list of names, or "all". Every distro runs by default: the
// scenarios are fully parallel, so the extra cost of the whole matrix is
// small.
const distroEnvVar = "YUPS_TEST_DISTRO"

// selectedDistros resolves the distros requested through distroEnvVar.
func selectedDistros() ([]distro, error) {
	selection := strings.ToLower(strings.TrimSpace(os.Getenv(distroEnvVar)))
	if selection == "" {
		selection = "all"
	}
	if selection == "all" {
		return allDistros, nil
	}

	var out []distro
	for _, name := range strings.Split(selection, ",") {
		name = strings.TrimSpace(name)
		known := false
		for _, d := range allDistros {
			if d.name == name {
				out = append(out, d)
				known = true
				break
			}
		}
		if !known {
			names := make([]string, len(allDistros))
			for i, d := range allDistros {
				names[i] = d.name
			}
			return nil, fmt.Errorf(
				"unknown distro %q in %s=%q (valid values: %s, or \"all\")",
				name, distroEnvVar, selection, strings.Join(names, ", "))
		}
	}
	return out, nil
}

var (
	testBinary  string   // path of the linux binary built once for all tests
	testDistros []distro // distros resolved at startup
)

func TestMain(m *testing.M) {
	os.Exit(runIntegration(m))
}

func runIntegration(m *testing.M) int {
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Println("docker not found in PATH; skipping integration tests")
		return 0
	}
	if err := exec.Command("docker", "info", "--format", "{{.ServerVersion}}").Run(); err != nil {
		fmt.Println("docker daemon unavailable; skipping integration tests:", err)
		return 0
	}

	tmpDir, err := os.MkdirTemp("", "yups-integration-*")
	if err != nil {
		fmt.Println("creating temp dir:", err)
		return 1
	}
	defer os.RemoveAll(tmpDir)

	testDistros, err = selectedDistros()
	if err != nil {
		fmt.Println(err)
		return 2
	}

	testBinary = filepath.Join(tmpDir, "yups")
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Println("cannot locate repository root")
		return 1
	}
	build := exec.Command("go", "build", "-o", testBinary, ".")
	build.Dir = filepath.Dir(filepath.Dir(thisFile)) // repository root
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Println("building yups failed:", err)
		return 1
	}

	// Pull every distro image up front, sequentially: when the parallel
	// suite starts, several first-time pulls race against each other and
	// the registry or daemon occasionally answers them with spurious
	// "page not found" errors or hangs outright (both seen on CI). Scenarios
	// run their containers with --pull=never, so this is the only place
	// where downloads happen; a final failure here means the affected
	// scenarios will fail fast instead of hanging.
	for _, d := range testDistros {
		var err error
		for attempt := 1; attempt <= 2; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			pull := exec.CommandContext(ctx, "docker", "pull", d.image)
			pull.Stdout, pull.Stderr = os.Stdout, os.Stderr
			err = pull.Run()
			cancel()
			if err == nil {
				break
			}
			fmt.Printf("warning: pulling %s failed on attempt %d: %v; retrying\n", d.image, attempt, err)
			time.Sleep(time.Duration(attempt) * 3 * time.Second)
		}
		if err != nil {
			fmt.Printf("warning: pre-pulling %s failed after retries (%v); its scenarios will fail fast\n", d.image, err)
		}
	}

	return m.Run()
}

// docker runs a docker command and returns its combined output and exit
// code. Non ExitError failures are fatal.
func docker(t *testing.T, args ...string) (string, int) {
	t.Helper()
	var buf bytes.Buffer
	cmd := exec.Command("docker", args...)
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()

	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, buf.String())
		}
		code = exitErr.ExitCode()
	}
	return buf.String(), code
}

// forEachImage runs fn once per selected distro, turning each run into a
// subtest named after it. Subtests run in parallel: every scenario uses its
// own throwaway container, so nothing is shared between them. The overall
// concurrency is bounded by `go test -parallel` (GOMAXPROCS by default).
func forEachImage(t *testing.T, fn func(t *testing.T, d distro)) {
	t.Helper()
	for _, d := range testDistros {
		d := d
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			fn(t, d)
		})
	}
}

// newContainer starts a fresh container of the given distro image with the
// built binary mounted read-only at /opt/yups and registers its removal.
// Extra docker run flags (for example --add-host) can be passed after the
// distro. The image is expected to be local already (TestMain pre-pulls
// it); --pull=never keeps network hiccups from hanging the suite: with no
// image available the scenario fails fast instead.
func newContainer(t *testing.T, d distro, extra ...string) string {
	t.Helper()
	args := append([]string{"run", "-d", "--pull=never", "--rm"}, extra...)
	args = append(args,
		"-v", testBinary+":/opt/yups:ro",
		d.image,
		"sleep", "900",
	)
	out, code := docker(t, args...)
	if code != 0 {
		t.Fatalf("starting %s container failed:\n%s", d.name, out)
	}
	id := strings.TrimSpace(out)
	t.Cleanup(func() { _, _ = docker(t, "rm", "-f", id) })
	if d.setup != "" {
		if out, code := sh(t, id, "root", "", d.setup); code != 0 {
			t.Fatalf("%s setup failed:\n%s", d.name, out)
		}
	}
	return id
}

// sh runs a shell script inside the container as user ("" means the image
// default, root) with an optional custom PATH.
func sh(t *testing.T, id, user, pathEnv, script string) (string, int) {
	t.Helper()
	return shWithInput(t, id, user, pathEnv, script, nil)
}

// shWithInput runs a shell script like sh, feeding input to its stdin
// (used to answer the interactive uninstall questions).
func shWithInput(t *testing.T, id, user, pathEnv, script string, stdin io.Reader) (string, int) {
	t.Helper()
	args := []string{"exec"}
	if stdin != nil {
		args = append(args, "-i")
	}
	if user != "" {
		args = append(args, "-u", user)
	}
	if pathEnv != "" {
		args = append(args, "-e", "PATH="+pathEnv)
	}
	args = append(args, id, "sh", "-c", script)

	var buf bytes.Buffer
	cmd := exec.Command("docker", args...)
	cmd.Stdout, cmd.Stderr = &buf, &buf
	cmd.Stdin = stdin
	err := cmd.Run()

	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, buf.String())
		}
		code = exitErr.ExitCode()
	}
	return buf.String(), code
}

// yupsCmd runs the mounted binary inside the container with the given
// argument.
func yupsCmd(t *testing.T, id, user, pathEnv, arg string) (string, int) {
	t.Helper()
	args := []string{"exec"}
	if user != "" {
		args = append(args, "-u", user)
	}
	if pathEnv != "" {
		args = append(args, "-e", "PATH="+pathEnv)
	}
	args = append(args, id, "/opt/yups")
	if arg != "" {
		args = append(args, arg)
	}
	return docker(t, args...)
}

// createUser creates a regular user inside the container, optionally adding
// it to the administrator group the distro ships by default (sudo on Ubuntu,
// wheel elsewhere). yups must recognise that group on its own.
func createUser(t *testing.T, id string, d distro, name string, sudo bool) {
	t.Helper()
	script := "useradd -m -s /bin/sh " + name
	if sudo {
		script += " && usermod -aG " + d.adminGroup + " " + name
	}
	if out, code := sh(t, id, "root", "", script); code != 0 {
		t.Fatalf("creating user %s failed:\n%s", name, out)
	}
}

// createWritableUserBin creates /home/<user>/bin owned by that user. Only
// the owner is set (no group): some distros (e.g. openSUSE) do not create a
// per-user group, so `chown user:user` would fail there.
func createWritableUserBin(t *testing.T, id, user string) {
	t.Helper()
	script := fmt.Sprintf("mkdir -p /home/%s/bin && chown %s /home/%s/bin", user, user, user)

	if out, code := sh(t, id, "root", "", script); code != 0 {
		t.Fatalf("creating writable bin dir for %s failed:\n%s", user, out)
	}
}

func fileExists(t *testing.T, id, path string) bool {
	_, code := sh(t, id, "root", "", "test -e "+path)
	return code == 0
}

func requireOutputContains(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q:\n%s", want, out)
		}
	}
}

func requireOutputNotContains(t *testing.T, out, not string) {
	t.Helper()
	if strings.Contains(out, not) {
		t.Errorf("output must not contain %q:\n%s", not, out)
	}
}

func TestPrintsColoredMarker(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		out, code := yupsCmd(t, id, "root", "", "")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}
		requireOutputContains(t, out, "\x1b[38;5;214m#_?\x1b[0m")
	})
}

func TestHelpListsCommands(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		out, code := yupsCmd(t, id, "root", "", "--help")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}
		requireOutputContains(t, out, "--help", "--install-yups", "--uninstall-yups")
	})
}

func TestUnknownOptionFailsWithUsageCode(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		_, code := yupsCmd(t, id, "root", "", "--nope")
		if code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
	})
}

func TestInstallAsRootInstallsIntoFirstPATHDirectory(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		out, code := yupsCmd(t, id, "root", "", "--install-yups")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}
		// Root can write everywhere; the first PATH directory wins.
		requireOutputContains(t, out, "/usr/local/sbin/yups")
		if !fileExists(t, id, "/usr/local/sbin/yups") {
			t.Error("/usr/local/sbin/yups was not created")
		}
		if !fileExists(t, id, "/root/.yups/config.toml") {
			t.Error("/root/.yups/config.toml was not initialized")
		}
	})
}

func TestInstallTwiceReportsAlreadyInstalled(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		if out, code := yupsCmd(t, id, "root", "", "--install-yups"); code != 0 {
			t.Fatalf("first install failed (%d):\n%s", code, out)
		}
		out, code := yupsCmd(t, id, "root", "", "--install-yups")
		if code != 0 {
			t.Fatalf("second install exit code = %d, want 0\n%s", code, out)
		}
		requireOutputContains(t, out, "already installed")
	})
}

func TestInstallDetectsSameNameCommandOutsideCurrentPATH(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		if out, code := sh(t, id, "root", "",
			"cp /opt/yups /usr/bin/yups && chmod 755 /usr/bin/yups && mkdir -p /tmp/only"); code != 0 {
			t.Fatalf("setup failed:\n%s", out)
		}
		// PATH no longer contains /usr/bin, but the command exists in one
		// of the well-known binary directories: step 2 of --install-yups
		// catches it.
		out, code := yupsCmd(t, id, "root", "/tmp/only", "--install-yups")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}
		requireOutputContains(t, out, "already installed", "/usr/bin/yups")
	})
}

func TestInstallWithoutPermissionsAndWithoutSudoFails(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		createUser(t, id, d, "plain", false)
		// The default PATH directories are all root-owned and not
		// writable by `plain`.
		out, code := yupsCmd(t, id, "plain", "", "--install-yups")
		if code == 0 {
			t.Fatalf("exit code = %d, want failure\n%s", code, out)
		}
		requireOutputContains(t, out, "write permissions")
		requireOutputNotContains(t, out, "sudo !!")
		if fileExists(t, id, "/usr/local/sbin/yups") || fileExists(t, id, "/usr/bin/yups") {
			t.Error("nothing should have been installed")
		}
	})
}

func TestInstallWithoutPermissionsAndWithSudoSuggestsSudoBangBang(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		createUser(t, id, d, "adminish", true)
		out, code := yupsCmd(t, id, "adminish", "", "--install-yups")
		if code == 0 {
			t.Fatalf("exit code = %d, want failure\n%s", code, out)
		}
		requireOutputContains(t, out, "sudo !!")
		if fileExists(t, id, "/usr/local/sbin/yups") {
			t.Error("nothing should have been installed")
		}
	})
}

func TestInstallIntoWritableHomeBin(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		createUser(t, id, d, "plain", false)
		createWritableUserBin(t, id, "plain")

		pathEnv := "/home/plain/bin:" + defaultPATH
		out, code := yupsCmd(t, id, "plain", pathEnv, "--install-yups")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}
		requireOutputContains(t, out, "/home/plain/bin/yups")
		if !fileExists(t, id, "/home/plain/bin/yups") {
			t.Error("/home/plain/bin/yups was not created")
		}
		if !fileExists(t, id, "/home/plain/.yups/config.toml") {
			t.Error("user config.toml was not created")
		}
		if _, code := sh(t, id, "root", "", "test -x /home/plain/bin/yups"); code != 0 {
			t.Error("installed file is not executable")
		}
	})
}

func TestUninstallWhenNotInstalled(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		out, code := yupsCmd(t, id, "root", "", "--uninstall-yups")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}
		requireOutputContains(t, out, "already uninstalled")
	})
}

func TestUninstallRemovesEveryCopy(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		if out, code := sh(t, id, "root", "",
			"cp /opt/yups /usr/local/sbin/yups && cp /opt/yups /usr/bin/yups"); code != 0 {
			t.Fatalf("setup failed:\n%s", out)
		}
		out, code := yupsCmd(t, id, "root", "", "--uninstall-yups")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}
		for _, path := range []string{"/usr/local/sbin/yups", "/usr/bin/yups"} {
			if fileExists(t, id, path) {
				t.Errorf("%s still exists", path)
			}
			requireOutputContains(t, out, "Removed "+path)
		}
	})
}

func TestUninstallBlockedWithoutSudoInformsUser(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		createUser(t, id, d, "plain", false)
		if out, code := sh(t, id, "root", "",
			"cp /opt/yups /usr/bin/yups && chmod 755 /usr/bin/yups"); code != 0 {
			t.Fatalf("setup failed:\n%s", out)
		}
		out, code := yupsCmd(t, id, "plain", "", "--uninstall-yups")
		if code == 0 {
			t.Fatalf("exit code = %d, want failure\n%s", code, out)
		}
		requireOutputContains(t, out, "do not have permissions")
		requireOutputNotContains(t, out, "sudo !!")
		if !fileExists(t, id, "/usr/bin/yups") {
			t.Error("/usr/bin/yups should have survived")
		}
	})
}

func TestUninstallBlockedWithSudoSuggestsSudoBangBang(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		createUser(t, id, d, "adminish", true)
		if out, code := sh(t, id, "root", "",
			"cp /opt/yups /usr/bin/yups && chmod 755 /usr/bin/yups"); code != 0 {
			t.Fatalf("setup failed:\n%s", out)
		}
		out, code := yupsCmd(t, id, "adminish", "", "--uninstall-yups")
		if code == 0 {
			t.Fatalf("exit code = %d, want failure\n%s", code, out)
		}
		requireOutputContains(t, out, "sudo !!")
		if !fileExists(t, id, "/usr/bin/yups") {
			t.Error("/usr/bin/yups should have survived until sudo is used")
		}
	})
}

func TestUninstallMixedPermissionsRemovesWritableAndHintsSudo(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		createUser(t, id, d, "adminish", true)
		createWritableUserBin(t, id, "adminish")
		setup := "cp /opt/yups /home/adminish/bin/yups && chown adminish /home/adminish/bin/yups"
		setup += " && cp /opt/yups /usr/bin/yups && chmod 755 /usr/bin/yups"
		if out, code := sh(t, id, "root", "", setup); code != 0 {
			t.Fatalf("setup failed:\n%s", out)
		}

		pathEnv := "/home/adminish/bin:" + defaultPATH
		out, code := yupsCmd(t, id, "adminish", pathEnv, "--uninstall-yups")
		if code == 0 {
			t.Fatalf("exit code = %d, want failure (some copies could not be removed)\n%s", code, out)
		}
		if fileExists(t, id, "/home/adminish/bin/yups") {
			t.Error("writable copy should have been removed")
		}
		if !fileExists(t, id, "/usr/bin/yups") {
			t.Error("/usr/bin/yups should have survived")
		}
		requireOutputContains(t, out, "Removed /home/adminish/bin/yups", "sudo !!")
	})
}

// --- Self-update scenarios -------------------------------------------------
//
// The release API is faked by an httptest server bound to all interfaces;
// containers reach it through the host-gateway alias "yups-release"
// (--add-host=yups-release:host-gateway, supported by every docker version
// this suite targets).

const releaseHostAlias = "yups-release"

// buildVersionedBinary builds the project binary with a version stamped
// through ldflags, like goreleaser does.
func buildVersionedBinary(t *testing.T, version string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	root := filepath.Dir(filepath.Dir(thisFile))
	bin := filepath.Join(t.TempDir(), "yups-"+strings.TrimPrefix(version, "v"))
	build := exec.Command("go", "build", "-o", bin,
		"-ldflags", "-X yups/internal/app.Version="+version, ".")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building %s: %v: %s", version, err, out)
	}
	return bin
}

// startReleaseServer serves a Forgejo-shaped releases/latest document plus
// the archive (payload packed as an executable named yups) and its
// checksums, on all interfaces so containers can connect.
func startReleaseServer(t *testing.T, tag string, payload []byte) string {
	t.Helper()

	archive := tarGzOf(t, payload)
	sum := sha256.Sum256(archive)
	archiveName := fmt.Sprintf("yups_%s_linux_%s.tar.gz", tag, runtime.GOARCH)

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("binding the fake release server: %v", err)
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	base := fmt.Sprintf("http://%s:%s", releaseHostAlias, port)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/repos/owner/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name": %q, "assets": [
			{"name": %q, "browser_download_url": "%s/archive.tar.gz"},
			{"name": "checksums.txt", "browser_download_url": "%s/checksums.txt"}]}`,
			tag, archiveName, base, base)
	})
	mux.HandleFunc("/archive.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), archiveName)
	})

	server := httptest.NewUnstartedServer(mux)
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)

	return base + "/owner/repo"
}

// tarGzOf packs payload as a root-level executable named yups.
func tarGzOf(t *testing.T, payload []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	writer := tar.NewWriter(gz)
	header := &tar.Header{Name: "yups", Mode: 0o755, Size: int64(len(payload)), Typeflag: tar.TypeReg}
	if err := writer.WriteHeader(header); err != nil {
		t.Fatalf("writing tar header: %v", err)
	}
	if _, err := writer.Write(payload); err != nil {
		t.Fatalf("writing tar payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("closing gzip: %v", err)
	}
	return buf.Bytes()
}

// installCopy places a binary into the container at path with executable
// permissions.
func installCopy(t *testing.T, id, source, path, owner string) {
	t.Helper()
	if out, code := docker(t, "cp", source, id+":"+path); code != 0 {
		t.Fatalf("copying %s into the container failed:\n%s", path, out)
	}
	script := "chmod 755 " + path
	if owner != "" {
		script += " && chown " + owner + " " + path
	}
	if out, code := sh(t, id, "root", "", script); code != 0 {
		t.Fatalf("preparing %s failed:\n%s", path, out)
	}
}

// writeUserConfig creates ~/.yups/config.toml for the given user pointing
// at the fake release repository.
func writeUserConfig(t *testing.T, id, user, repoURL, version string) {
	t.Helper()
	home := "/root"
	if user != "" && user != "root" {
		home = "/home/" + user
	}
	script := fmt.Sprintf(
		"mkdir -p %[1]s/.yups && printf 'version = \"%[2]s\"\\nYUPS_REPO = \"%[3]s\"\\nYUPS_REPO_FALLBACK = \"http://fallback.invalid/a/b\"\\n' > %[1]s/.yups/config.toml",
		home, version, repoURL)
	asUser := ""
	if user != "" && user != "root" {
		asUser = user
	}
	if out, code := sh(t, id, asUser, "", script); code != 0 {
		t.Fatalf("writing the config of %s failed:\n%s", user, out)
	}
}

func TestUpdateInsideContainerAsRoot(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		oldBin := buildVersionedBinary(t, "v0.0.9")
		newBin := buildVersionedBinary(t, "v0.1.0")
		payload, err := os.ReadFile(newBin)
		if err != nil {
			t.Fatalf("reading the new binary: %v", err)
		}
		repoURL := startReleaseServer(t, "v0.1.0", payload)

		id := newContainer(t, d, "--add-host="+releaseHostAlias+":host-gateway")
		installCopy(t, id, oldBin, "/usr/local/bin/yups", "")
		writeUserConfig(t, id, "root", repoURL, "v0.0.9")

		out, code := sh(t, id, "root", "", "/usr/local/bin/yups --update-yups")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}
		requireOutputContains(t, out, "updated to v0.1.0")

		versionOut, code := sh(t, id, "root", "", "/usr/local/bin/yups --version")
		if code != 0 || !strings.Contains(versionOut, "v0.1.0") {
			t.Errorf("the installed binary does not report v0.1.0 (%d): %s", code, versionOut)
		}
		configOut, _ := sh(t, id, "root", "", "cat /root/.yups/config.toml")
		requireOutputContains(t, configOut, `version = "v0.1.0"`)
	})
}

func TestUpdateInsideContainerAsPlainUser(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		oldBin := buildVersionedBinary(t, "v0.0.9")
		newBin := buildVersionedBinary(t, "v0.1.0")
		payload, err := os.ReadFile(newBin)
		if err != nil {
			t.Fatalf("reading the new binary: %v", err)
		}
		repoURL := startReleaseServer(t, "v0.1.0", payload)

		id := newContainer(t, d, "--add-host="+releaseHostAlias+":host-gateway")
		createUser(t, id, d, "plain", false)
		createWritableUserBin(t, id, "plain")
		installCopy(t, id, oldBin, "/home/plain/bin/yups", "plain")
		writeUserConfig(t, id, "plain", repoURL, "v0.0.9")

		pathEnv := "/home/plain/bin:" + defaultPATH
		out, code := sh(t, id, "plain", pathEnv, "yups --update-yups")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}
		requireOutputContains(t, out, "updated to v0.1.0")

		versionOut, code := sh(t, id, "plain", pathEnv, "yups --version")
		if code != 0 || !strings.Contains(versionOut, "v0.1.0") {
			t.Errorf("the installed binary does not report v0.1.0 (%d): %s", code, versionOut)
		}
	})
}

func TestUpdateAsRootDoesNotTouchOtherUsersConfig(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		oldBin := buildVersionedBinary(t, "v0.0.9")
		newBin := buildVersionedBinary(t, "v0.1.0")
		payload, err := os.ReadFile(newBin)
		if err != nil {
			t.Fatalf("reading the new binary: %v", err)
		}
		repoURL := startReleaseServer(t, "v0.1.0", payload)

		id := newContainer(t, d, "--add-host="+releaseHostAlias+":host-gateway")
		createUser(t, id, d, "plain", false)
		installCopy(t, id, oldBin, "/usr/local/bin/yups", "")
		writeUserConfig(t, id, "root", repoURL, "v0.0.9")
		writeUserConfig(t, id, "plain", "http://nobody.invalid/a/b", "v0.0.5")

		out, code := sh(t, id, "root", "", "/usr/local/bin/yups --update-yups")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}

		rootConfig, _ := sh(t, id, "root", "", "cat /root/.yups/config.toml")
		requireOutputContains(t, rootConfig, `version = "v0.1.0"`)
		plainConfig, _ := sh(t, id, "root", "", "cat /home/plain/.yups/config.toml")
		requireOutputContains(t, plainConfig, `version = "v0.0.5"`)
	})
}

func TestUninstallKeepsConfigByDefault(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		if out, code := yupsCmd(t, id, "root", "", "--install-yups"); code != 0 {
			t.Fatalf("install failed (%d):\n%s", code, out)
		}
		writeUserConfig(t, id, "root", "http://nobody.invalid/a/b", "v0.0.9")

		// docker exec without -i leaves stdin closed: the questions fall
		// back to their defaults (uninstall for all users, keep ~/.yups).
		out, code := yupsCmd(t, id, "root", "", "--uninstall-yups")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}
		if fileExists(t, id, "/usr/local/sbin/yups") || fileExists(t, id, "/usr/local/bin/yups") {
			t.Error("the binary should have been removed")
		}
		if !fileExists(t, id, "/root/.yups/config.toml") {
			t.Error("~/.yups/config.toml should have survived by default")
		}
		requireOutputContains(t, out, "Keeping /root/.yups")
	})
}

func TestUninstallDeletesConfigWhenConfirmed(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		if out, code := yupsCmd(t, id, "root", "", "--install-yups"); code != 0 {
			t.Fatalf("install failed (%d):\n%s", code, out)
		}
		writeUserConfig(t, id, "root", "http://nobody.invalid/a/b", "v0.0.9")

		// "n" declines keeping the state directory, "y" confirms deletion for all users.
		out, code := shWithInput(t, id, "root", "", "/opt/yups --uninstall-yups", strings.NewReader("n\ny\n"))
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}
		if fileExists(t, id, "/usr/local/sbin/yups") || fileExists(t, id, "/usr/local/bin/yups") {
			t.Error("the binary should have been removed")
		}
		if fileExists(t, id, "/root/.yups") {
			t.Error("~/.yups should have been deleted after confirmation")
		}
		requireOutputContains(t, out, "Deleted /root/.yups")
	})
}

func TestExplainCommandInsideContainers(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		out, code := sh(t, id, "root", "", "/opt/yups -- ls -a /root")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}
		requireOutputContains(t, out, "#_?")
		requireOutputContains(t, out, "Found: ls")
		requireOutputContains(t, out, "-a found:")
	})
}
