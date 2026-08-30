// update_e2e_test.go - end-to-end self-update test with no network and no
// docker: the real binary is built twice with different ldflags (old and
// new), an httptest server plays the release API, and the old binary is
// actually executed so phase 1 hands over to phase 2 through a real
// syscall.Exec.
package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"yups/internal/config"
	"yups/internal/update"
)

func TestUpdateEndToEndReplacesInstalledBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping the end-to-end update test in -short mode")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain not available: %v", err)
	}

	// Repo root: this file lives in <root>/internal/app/.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine the repository root")
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))

	workDir := t.TempDir()
	oldBinary := filepath.Join(workDir, "yups-old")
	newBinary := filepath.Join(workDir, "yups-new")
	for item, version := range map[string]string{
		oldBinary: "v0.0.9",
		newBinary: "v0.1.0",
	} {
		build := exec.Command(goTool, "build", "-o", item,
			"-ldflags", "-X yups/internal/app.Version="+version, ".")
		build.Dir = repoRoot
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("building %s: %v: %s", version, err, out)
		}
	}
	newPayload, err := os.ReadFile(newBinary)
	if err != nil {
		t.Fatalf("reading the new binary: %v", err)
	}

	// Release API double: Forgejo-shaped latest endpoint plus the archive
	// (containing the new binary) and its checksums.
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/owner/repo/releases/latest":
			fmt.Fprintf(w, `{"tag_name": "v0.1.0", "assets": [
				{"name": "yups-v0.1.0-linux-%s.tar.gz", "browser_download_url": "%s/archive.tar.gz"},
				{"name": %q, "browser_download_url": "%s/checksums.txt"}]}`,
				runtime.GOARCH, server.URL, update.ChecksumsFileName, server.URL)
		case "/archive.tar.gz":
			// Test double: a failed write fails the scenario anyway.
			_, _ = w.Write(buildTestArchive(t, newPayload))
		case "/checksums.txt":
			sum := sha256.Sum256(buildTestArchive(t, newPayload))
			fmt.Fprintf(w, "%s  yups-v0.1.0-linux-%s.tar.gz\n", hex.EncodeToString(sum[:]), runtime.GOARCH)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Fake user environment: fake HOME with a config pointing at the test
	// server, and the old binary installed into the only PATH directory.
	home := filepath.Join(workDir, "home")
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("creating fake home: %v", err)
	}
	configBody := fmt.Sprintf("yups-repo = %q\nyups-repo-fallback = %q\n",
		server.URL+"/owner/repo", server.URL+"/fallback/repo")
	if err := os.MkdirAll(config.Dir(home), 0o755); err != nil {
		t.Fatalf("creating the fake .yups directory: %v", err)
	}
	if err := os.WriteFile(config.Path(home), []byte(configBody), 0o644); err != nil {
		t.Fatalf("writing the fake config: %v", err)
	}
	stateBody := "version = \"v0.0.9\"\nlast-applied = \"v0.0.9\"\n"
	if err := os.WriteFile(config.StatePath(home), []byte(stateBody), 0o644); err != nil {
		t.Fatalf("writing the fake state: %v", err)
	}
	installedBinary := filepath.Join(binDir, ProgramName)
	payload, err := os.ReadFile(oldBinary)
	if err != nil {
		t.Fatalf("reading the old binary: %v", err)
	}
	if err := os.WriteFile(installedBinary, payload, 0o755); err != nil {
		t.Fatalf("installing the old binary: %v", err)
	}

	childEnv := []string{"HOME=" + home, "PATH=" + binDir}
	update := exec.Command(oldBinary, "--update-yups")
	update.Env = childEnv
	out, err := update.CombinedOutput()
	if err != nil {
		t.Fatalf("running --update-yups: %v: %s", err, out)
	}
	if !strings.Contains(string(out), "updated to v0.1.0") {
		t.Errorf("update output does not announce the new version: %s", out)
	}

	versionRun := exec.Command(installedBinary, "--version")
	versionRun.Env = childEnv
	versionOut, err := versionRun.CombinedOutput()
	if err != nil {
		t.Fatalf("running the updated binary: %v: %s", err, versionOut)
	}
	if got := strings.TrimSpace(string(versionOut)); got != ProgramName+" v0.1.0" {
		t.Errorf("installed binary reports %q, want %q", got, ProgramName+" v0.1.0")
	}

	updatedState, err := os.ReadFile(config.StatePath(home))
	if err != nil {
		t.Fatalf("reading the state after the update: %v", err)
	}
	if !strings.Contains(string(updatedState), `version = "v0.1.0"`) {
		t.Errorf("state.version did not advance: %s", updatedState)
	}
}

// buildTestArchive packs payload as a root-level executable named yups.
func buildTestArchive(t *testing.T, payload []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	writer := tar.NewWriter(gz)

	header := &tar.Header{Name: ProgramName, Mode: 0o755, Size: int64(len(payload)), Typeflag: tar.TypeReg}
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
