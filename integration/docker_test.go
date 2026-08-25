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
	"bytes"
	"context"
	"fmt"
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
// The image is expected to be local already (TestMain pre-pulls it);
// --pull=never keeps network hiccups from hanging the suite: with no image
// available the scenario fails fast instead.
func newContainer(t *testing.T, d distro) string {
	t.Helper()
	out, code := docker(t,
		"run", "-d", "--pull=never", "--rm",
		"-v", testBinary+":/opt/yups:ro",
		d.image,
		"sleep", "900",
	)
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
	args := []string{"exec"}
	if user != "" {
		args = append(args, "-u", user)
	}
	if pathEnv != "" {
		args = append(args, "-e", "PATH="+pathEnv)
	}
	return docker(t, append(args, id, "sh", "-c", script)...)
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
		requireOutputContains(t, out, "--help", "--install", "--uninstall")
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
		out, code := yupsCmd(t, id, "root", "", "--install")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}
		// Root can write everywhere; the first PATH directory wins.
		requireOutputContains(t, out, "/usr/local/sbin/yups")
		if !fileExists(t, id, "/usr/local/sbin/yups") {
			t.Error("/usr/local/sbin/yups was not created")
		}
	})
}

func TestInstallTwiceReportsAlreadyInstalled(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		if out, code := yupsCmd(t, id, "root", "", "--install"); code != 0 {
			t.Fatalf("first install failed (%d):\n%s", code, out)
		}
		out, code := yupsCmd(t, id, "root", "", "--install")
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
		// of the well-known binary directories: step 2 of --install
		// catches it.
		out, code := yupsCmd(t, id, "root", "/tmp/only", "--install")
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
		out, code := yupsCmd(t, id, "plain", "", "--install")
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
		out, code := yupsCmd(t, id, "adminish", "", "--install")
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
		out, code := yupsCmd(t, id, "plain", pathEnv, "--install")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\n%s", code, out)
		}
		requireOutputContains(t, out, "/home/plain/bin/yups")
		if !fileExists(t, id, "/home/plain/bin/yups") {
			t.Error("/home/plain/bin/yups was not created")
		}
		if _, code := sh(t, id, "root", "", "test -x /home/plain/bin/yups"); code != 0 {
			t.Error("installed file is not executable")
		}
	})
}

func TestUninstallWhenNotInstalled(t *testing.T) {
	forEachImage(t, func(t *testing.T, d distro) {
		id := newContainer(t, d)
		out, code := yupsCmd(t, id, "root", "", "--uninstall")
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
		out, code := yupsCmd(t, id, "root", "", "--uninstall")
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
		out, code := yupsCmd(t, id, "plain", "", "--uninstall")
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
		out, code := yupsCmd(t, id, "adminish", "", "--uninstall")
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
		out, code := yupsCmd(t, id, "adminish", pathEnv, "--uninstall")
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
