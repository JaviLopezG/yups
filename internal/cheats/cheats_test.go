package cheats

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("cannot create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("cannot write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("cannot close zip writer: %v", err)
	}
	return buf.Bytes()
}

func createTestTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for name, content := range files {
		data := []byte(content)
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("cannot write tar header %s: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("cannot write tar entry %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("cannot close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("cannot close gzip writer: %v", err)
	}
	return buf.Bytes()
}

func TestExtractZip(t *testing.T) {
	tempDir := t.TempDir()
	zipData := createTestZip(t, map[string]string{
		"pages/common/tar.md": "# tar\n> Archiving utility.\n",
		"pages/linux/ip.md":   "# ip\n> Network interface tool.\n",
	})

	if err := extractZip(zipData, tempDir); err != nil {
		t.Fatalf("extractZip failed: %v", err)
	}

	tarMd := filepath.Join(tempDir, "pages/common/tar.md")
	content, err := os.ReadFile(tarMd)
	if err != nil {
		t.Fatalf("cannot read extracted tar.md: %v", err)
	}
	if !strings.Contains(string(content), "Archiving utility") {
		t.Errorf("content does not contain expected text: %s", string(content))
	}
}

func TestExtractZipRejectsPathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	zipData := createTestZip(t, map[string]string{
		"../escaped.txt": "evil",
	})

	if err := extractZip(zipData, tempDir); err == nil {
		t.Fatal("expected error on path traversal in zip, got nil")
	}
}

func TestExtractTarGz(t *testing.T) {
	tempDir := t.TempDir()
	tarData := createTestTarGz(t, map[string]string{
		"sheets/tar":        "tar cheatsheet content\n",
		"see_also/find.txt": "find cheatsheet content\n",
	})

	if err := extractTarGz(tarData, tempDir); err != nil {
		t.Fatalf("extractTarGz failed: %v", err)
	}

	tarSheet := filepath.Join(tempDir, "sheets/tar")
	content, err := os.ReadFile(tarSheet)
	if err != nil {
		t.Fatalf("cannot read extracted tar sheet: %v", err)
	}
	if !strings.Contains(string(content), "tar cheatsheet content") {
		t.Errorf("content mismatch: %s", string(content))
	}
}

func TestExtractTarGzRejectsPathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	tarData := createTestTarGz(t, map[string]string{
		"../../evil.txt": "evil",
	})

	if err := extractTarGz(tarData, tempDir); err == nil {
		t.Fatal("expected error on path traversal in tar.gz, got nil")
	}
}

func TestFindCheatsheets(t *testing.T) {
	baseDir := t.TempDir()

	// Setup mock structure for tldr, navi, cheat-sh, cheat
	tldrDir := filepath.Join(baseDir, "tldr", "pages", "common")
	_ = os.MkdirAll(tldrDir, 0755)
	_ = os.WriteFile(filepath.Join(tldrDir, "tar.md"), []byte("# tar tldr"), 0644)
	_ = os.WriteFile(filepath.Join(tldrDir, "git-commit.md"), []byte("# git commit tldr"), 0644)

	naviDir := filepath.Join(baseDir, "navi", "code")
	_ = os.MkdirAll(naviDir, 0755)
	_ = os.WriteFile(filepath.Join(naviDir, "tar.cheat"), []byte("% tar, archive\n# Extract tar"), 0644)

	cheatShDir := filepath.Join(baseDir, "cheat-sh", "sheets")
	_ = os.MkdirAll(cheatShDir, 0755)
	_ = os.WriteFile(filepath.Join(cheatShDir, "tar"), []byte("cheat-sh tar"), 0644)

	cheatDir := filepath.Join(baseDir, "cheat", "cheatsheets-master")
	_ = os.MkdirAll(cheatDir, 0755)
	_ = os.WriteFile(filepath.Join(cheatDir, "tar"), []byte("cheat tar"), 0644)

	// Search for 'tar'
	entries := FindCheatsheets(baseDir, "tar", "")
	if len(entries) < 4 {
		t.Errorf("len(entries) = %d, want at least 4 (found: %+v)", len(entries), entries)
	}

	sources := make(map[string]bool)
	for _, e := range entries {
		sources[e.Source] = true
	}
	for _, wantSrc := range []string{"tldr", "navi", "cheat-sh", "cheat"} {
		if !sources[wantSrc] {
			t.Errorf("missing source %q in results", wantSrc)
		}
	}

	// Search for 'git commit'
	subEntries := FindCheatsheets(baseDir, "git", "commit")
	if len(subEntries) == 0 {
		t.Error("expected to find git-commit cheatsheet")
	}
}

func TestDownloadAllWithMockServer(t *testing.T) {
	zipData := createTestZip(t, map[string]string{
		"pages/common/tar.md": "# tar tldr",
	})
	tarData := createTestTarGz(t, map[string]string{
		"tar": "tar cheat",
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".zip") {
			_, _ = w.Write(zipData)
			return
		}
		_, _ = w.Write(tarData)
	}))
	defer ts.Close()

	// Override DefaultSources for the test
	origSources := DefaultSources
	defer func() { DefaultSources = origSources }()

	DefaultSources = []Source{
		{
			ID:     "tldr",
			Name:   "tldr-pages",
			URL:    ts.URL + "/tldr.zip",
			Format: FormatZip,
			Credit: "Thanks tldr",
		},
		{
			ID:     "cheat",
			Name:   "cheat",
			URL:    ts.URL + "/cheat.tar.gz",
			Format: FormatTarGz,
			Credit: "Thanks cheat",
		},
	}

	destDir := t.TempDir()
	var stdout bytes.Buffer
	err := DownloadAll(ts.Client(), destDir, &stdout)
	if err != nil {
		t.Fatalf("DownloadAll failed: %v", err)
	}

	outStr := stdout.String()
	if !strings.Contains(outStr, "Thanks tldr") || !strings.Contains(outStr, "Thanks cheat") {
		t.Errorf("expected attribution in stdout, got:\n%s", outStr)
	}

	tldrFile := filepath.Join(destDir, "tldr", "pages", "common", "tar.md")
	if _, err := os.Stat(tldrFile); err != nil {
		t.Errorf("expected %s to exist: %v", tldrFile, err)
	}
}
