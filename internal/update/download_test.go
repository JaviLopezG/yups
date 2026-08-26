package update

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
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchReturnsBodyAndRejectsErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "payload")
	}))
	defer server.Close()

	data, err := Fetch(server.Client(), server.URL)
	if err != nil || string(data) != "payload" {
		t.Fatalf("Fetch = %q, %v; want payload, nil", data, err)
	}

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer broken.Close()

	if _, err := Fetch(broken.Client(), broken.URL); err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("Fetch error = %v, want a 500 status mention", err)
	}
}

func TestVerifyChecksums(t *testing.T) {
	payload := []byte("the new binary")
	sum := sha256.Sum256(payload)
	good := hex.EncodeToString(sum[:])

	tests := []struct {
		name      string
		checksums string
		fileName  string
		data      []byte
		wantErr   string
	}{
		{
			name:      "matching entry passes",
			checksums: good + "  yups_v1.0.0_linux_amd64.tar.gz\n",
			fileName:  "yups_v1.0.0_linux_amd64.tar.gz",
			data:      payload,
		},
		{
			name:      "uppercase hex passes",
			checksums: strings.ToUpper(good) + "  file.bin\n",
			fileName:  "file.bin",
			data:      payload,
		},
		{
			name:      "mismatched digest fails",
			checksums: strings.Repeat("ab", 32) + "  file.bin\n",
			fileName:  "file.bin",
			data:      payload,
			wantErr:   "checksum mismatch",
		},
		{
			name:      "missing entry fails",
			checksums: good + "  some-other-file\n",
			fileName:  "file.bin",
			data:      payload,
			wantErr:   "no entry for file.bin",
		},
		{
			name:      "malformed lines are skipped without hiding the entry",
			checksums: "garbage line\n\n" + good + "  file.bin\n",
			fileName:  "file.bin",
			data:      payload,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyChecksums([]byte(tt.checksums), tt.fileName, tt.data)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("VerifyChecksums: unexpected error %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("VerifyChecksums error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// tarEntry is one file (or directory, when the name ends in /) of a
// synthetic archive.
type tarEntry struct {
	name    string
	content string
	mode    int64
}

// tarGz builds an in-memory tar.gz archive from name/content/mode entries.
func tarGz(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	writer := tar.NewWriter(gz)

	for _, e := range entries {
		header := &tar.Header{Name: e.name, Mode: e.mode, Size: int64(len(e.content))}
		var typeflag byte
		if strings.HasSuffix(e.name, "/") {
			typeflag = tar.TypeDir
			header.Size = 0
		} else {
			typeflag = tar.TypeReg
		}
		header.Typeflag = typeflag
		if err := writer.WriteHeader(header); err != nil {
			t.Fatalf("writing header for %s: %v", e.name, err)
		}
		if typeflag == tar.TypeReg {
			if _, err := writer.Write([]byte(e.content)); err != nil {
				t.Fatalf("writing content of %s: %v", e.name, err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("closing gzip: %v", err)
	}
	return buf.Bytes()
}

func TestExtractTarGzExtractsFilesWithModes(t *testing.T) {
	dest := t.TempDir()
	archive := tarGz(t,
		tarEntry{name: "bin/", mode: 0o755},
		tarEntry{name: "bin/yups", content: "#!/bin/sh\n", mode: 0o755},
		tarEntry{name: "README.md", content: "hello", mode: 0o644},
	)

	if err := ExtractTarGz(archive, dest); err != nil {
		t.Fatalf("ExtractTarGz: %v", err)
	}

	binPath := filepath.Join(dest, "bin", "yups")
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("extracted binary missing: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("binary mode = %o, want 755", info.Mode().Perm())
	}
	readme, err := os.ReadFile(filepath.Join(dest, "README.md"))
	if err != nil || string(readme) != "hello" {
		t.Errorf("README.md = %q, %v; want hello, nil", readme, err)
	}
}

func TestExtractTarGzRejectsEscapingEntries(t *testing.T) {
	outside := t.TempDir()
	dest := filepath.Join(outside, "dest")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	for _, tt := range []struct {
		name    string
		archive []byte
	}{
		{"absolute path", tarGz(t, tarEntry{name: "/tmp/yups-evil", content: "x", mode: 0o644})},
		{"parent traversal", tarGz(t, tarEntry{name: "../yups-evil", content: "x", mode: 0o644})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ExtractTarGz(tt.archive, dest)
			if err == nil || !strings.Contains(err.Error(), "escapes") {
				t.Fatalf("ExtractTarGz error = %v, want an escape rejection", err)
			}
			matches, _ := filepath.Glob(filepath.Join(outside, "yups-evil"))
			if len(matches) != 0 {
				t.Errorf("escaping entry was written to %v", matches)
			}
		})
	}
}

func TestExtractTarGzRejectsNonGzipInput(t *testing.T) {
	if err := ExtractTarGz([]byte("plain text"), t.TempDir()); err == nil {
		t.Fatal("ExtractTarGz on plain text: want error, got nil")
	}
}

// writeScript writes an executable shell script and returns its path.
func writeScript(t *testing.T, dir, body string) string {
	t.Helper()

	path := filepath.Join(dir, "script.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("writing script: %v", err)
	}
	return path
}

func TestValidateBinary(t *testing.T) {
	dir := t.TempDir()

	passing := writeScript(t, dir, `echo "yups v1.2.3"`)
	if err := ValidateBinary(passing, "yups", "v1.2.3"); err != nil {
		t.Errorf("ValidateBinary passing case: %v", err)
	}

	wrongVersion := writeScript(t, dir, `echo "yups v0.0.1"`)
	err := ValidateBinary(wrongVersion, "yups", "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "self-check failed") {
		t.Errorf("wrong version error = %v, want a self-check failure", err)
	}

	failing := writeScript(t, dir, `echo boom >&2; exit 3`)
	err = ValidateBinary(failing, "yups", "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "failed") {
		t.Errorf("failing binary error = %v, want an execution failure", err)
	}
}
