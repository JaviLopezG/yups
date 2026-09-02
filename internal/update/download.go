// download.go - fetch release assets, verify them against the published
// checksums, extract the archive and self-validate the new binary.
package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// defaultTimeout bounds every network round trip and the binary
// self-check; an update must never hang the user's terminal.
const defaultTimeout = 30 * time.Second

// Fetch downloads url with the given client (a default client with a
// timeout is used when nil) and returns the body bytes.
func Fetch(client *http.Client, url string) ([]byte, error) {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: unexpected status %q", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// VerifyChecksums checks that data matches the sha256 entry of fileName in
// a goreleaser checksums.txt document ("<hex>  <name>" per line). Lines
// with an unexpected shape are skipped: only an exact-name entry matters,
// and its absence is an error.
func VerifyChecksums(checksums []byte, fileName string, data []byte) error {
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])

	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 || fields[1] != fileName {
			continue
		}
		if !strings.EqualFold(fields[0], actual) {
			return fmt.Errorf("checksum mismatch for %s: downloaded %s, checksums.txt says %s", fileName, actual, fields[0])
		}
		return nil
	}
	return fmt.Errorf("%s has no entry for %s", ChecksumsFileName, fileName)
}

// ExtractTarGz unpacks the release archive into destDir. Entries whose
// name would escape destDir (absolute paths or .. elements) are rejected:
// the archive comes from the network even after the checksum check.
func ExtractTarGz(archive []byte, destDir string) error {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("opening gzip archive: %w", err)
	}
	defer gz.Close()

	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading archive: %w", err)
		}

		name := filepath.Clean(header.Name)
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("archive entry %q escapes the destination directory", header.Name)
		}
		target := filepath.Join(destDir, name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, fs.FileMode(header.Mode)&0o777); err != nil {
				return fmt.Errorf("creating directory %q: %w", target, err)
			}
		case tar.TypeReg:
			if err := extractFile(reader, target, fs.FileMode(header.Mode)); err != nil {
				return err
			}
		default:
			// Release archives only ever contain directories and regular
			// files; anything else is ignored rather than trusted.
			continue
		}
	}
}

func extractFile(reader *tar.Reader, target string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("creating parent of %q: %w", target, err)
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode&0o777)
	if err != nil {
		return fmt.Errorf("creating %q: %w", target, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("extracting %q: %w", target, err)
	}
	// OpenFile applies the umask to the creation mode; restore the exact
	// archived bits so executables stay executable.
	if err := os.Chmod(target, mode&0o777); err != nil {
		return fmt.Errorf("setting mode of %q: %w", target, err)
	}
	return nil
}

// ValidateBinary runs path --version and requires the output to be exactly
// "<programName> <tag>": the staged binary must prove it really is the
// release it claims to be before anything replaces the installed one.
func ValidateBinary(path, programName, tag string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	output, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("self-check %s --version failed: %w: %s", path, err, strings.TrimSpace(string(output)))
	}
	want := programName + " " + tag
	found := false
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == want {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("self-check failed: %s --version reported %q, want %q", path, strings.TrimSpace(string(output)), want)
	}
	return nil
}
