package cheats

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func createDummyZip() []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("tar.md")
	_, _ = f.Write([]byte("# tar cheatsheet\n> Archive utility\n"))
	_ = w.Close()
	return buf.Bytes()
}

func TestSyncConditionalDownloadingWithWeeklyTTL(t *testing.T) {
	zipData := createDummyZip()
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zipData)
	}))
	defer server.Close()

	origSources := DefaultSources
	DefaultSources = []Source{
		{
			ID:     "tldr",
			Name:   "tldr-pages",
			URL:    server.URL + "/tldr.zip",
			Format: FormatZip,
			Credit: "Test Credit",
		},
	}
	defer func() { DefaultSources = origSources }()

	destDir := t.TempDir()
	var stdout bytes.Buffer

	// 1. Initial Sync without cache -> Downloads and extracts
	newVersions, err := Sync(server.Client(), destDir, nil, &stdout)
	if err != nil {
		t.Fatalf("Sync initial failed: %v", err)
	}
	if newVersions["tldr"] == "" {
		t.Errorf("expected recorded timestamp for tldr, got empty")
	}
	if requestCount != 1 {
		t.Errorf("expected 1 HTTP request, got %d", requestCount)
	}
	if !strings.Contains(stdout.String(), "Downloading cheatsheets") {
		t.Errorf("stdout missing download message:\n%s", stdout.String())
	}

	extractedFile := filepath.Join(destDir, "tldr", "tar.md")
	if _, err := os.Stat(extractedFile); err != nil {
		t.Fatalf("expected extracted file %s to exist", extractedFile)
	}

	// 2. Second Sync with fresh timestamp (< 7 days) -> Skips download without any network call
	stdout.Reset()
	updatedVersions, err := Sync(server.Client(), destDir, newVersions, &stdout)
	if err != nil {
		t.Fatalf("Sync second time failed: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("expected NO new HTTP requests (0 network calls), got count %d", requestCount)
	}
	if !strings.Contains(stdout.String(), "are up to date") {
		t.Errorf("stdout missing 'up to date' message:\n%s", stdout.String())
	}
	if updatedVersions["tldr"] != newVersions["tldr"] {
		t.Errorf("timestamp changed unexpectedly: %v != %v", updatedVersions["tldr"], newVersions["tldr"])
	}

	// 3. Third Sync with expired timestamp (> 7 days) -> Triggers re-download
	stdout.Reset()
	oldTime := time.Now().Add(-10 * 24 * time.Hour).UTC().Format(time.RFC3339)
	staleVersions := map[string]string{"tldr": oldTime, "last-updated": oldTime}

	refreshedVersions, err := Sync(server.Client(), destDir, staleVersions, &stdout)
	if err != nil {
		t.Fatalf("Sync third time failed: %v", err)
	}
	if requestCount != 2 {
		t.Errorf("expected 2nd HTTP request on expired cache, got %d", requestCount)
	}
	if !strings.Contains(stdout.String(), "Downloading cheatsheets") {
		t.Errorf("expected download message for stale cache, got:\n%s", stdout.String())
	}
	if refreshedVersions["tldr"] == oldTime {
		t.Errorf("expected timestamp to be refreshed to current time")
	}
}

func TestSyncRedownloadsWhenLocalDirectoryMissing(t *testing.T) {
	zipData := createDummyZip()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zipData)
	}))
	defer server.Close()

	origSources := DefaultSources
	DefaultSources = []Source{
		{
			ID:     "tldr",
			Name:   "tldr-pages",
			URL:    server.URL + "/tldr.zip",
			Format: FormatZip,
			Credit: "Test Credit",
		},
	}
	defer func() { DefaultSources = origSources }()

	destDir := t.TempDir()
	nowStr := time.Now().UTC().Format(time.RFC3339)
	cached := map[string]string{"tldr": nowStr, "last-updated": nowStr}
	var stdout bytes.Buffer

	// Target dir does NOT exist on disk, so it must download even with recent timestamp
	newVersions, err := Sync(server.Client(), destDir, cached, &stdout)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("expected HTTP request when files missing, got %d", requestCount)
	}
	if !strings.Contains(stdout.String(), "Downloading cheatsheets") {
		t.Errorf("expected download when directory missing, got stdout:\n%s", stdout.String())
	}
	if newVersions["tldr"] == "" {
		t.Errorf("expected timestamp recorded, got empty")
	}
}

func TestDownloadAllWrapper(t *testing.T) {
	zipData := createDummyZip()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zipData)
	}))
	defer server.Close()

	origSources := DefaultSources
	DefaultSources = []Source{
		{
			ID:     "tldr",
			Name:   "tldr-pages",
			URL:    server.URL + "/tldr.zip",
			Format: FormatZip,
			Credit: "Test Credit",
		},
	}
	defer func() { DefaultSources = origSources }()

	destDir := t.TempDir()
	if err := DownloadAll(server.Client(), destDir, io.Discard); err != nil {
		t.Fatalf("DownloadAll failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "tldr", "tar.md")); err != nil {
		t.Fatalf("expected extracted file to exist")
	}
}
