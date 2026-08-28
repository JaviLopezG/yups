// Package cheats handles fetching, extracting, and querying community
// cheatsheet collections (tldr-pages, navi, cheat.sh, and cheat).
package cheats

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SourceFormat specifies the archive format.
type SourceFormat string

const (
	FormatZip   SourceFormat = "zip"
	FormatTarGz SourceFormat = "tar.gz"
)

// Source represents an external cheatsheet repository with attribution.
type Source struct {
	ID     string       // Subdirectory identifier under cheatsheets/
	Name   string       // Human-readable source name
	URL    string       // Archive download URL
	Format SourceFormat // Archive format
	Credit string       // Acknowledgment message to display
}

// DefaultSources defines the four community cheatsheet repositories.
var DefaultSources = []Source{
	{
		ID:     "tldr",
		Name:   "tldr-pages",
		URL:    "https://github.com/tldr-pages/tldr/releases/download/v2.3/tldr-pages.en.zip",
		Format: FormatZip,
		Credit: "Special thanks to the tldr-pages project and contributors! (https://github.com/tldr-pages/tldr)",
	},
	{
		ID:     "navi",
		Name:   "navi cheatsheets",
		URL:    "https://github.com/denisidoro/cheats/archive/refs/heads/master.tar.gz",
		Format: FormatTarGz,
		Credit: "Special thanks to Denis Isidoro and navi contributors! (https://github.com/denisidoro/cheats)",
	},
	{
		ID:     "cheat-sh",
		Name:   "cheat.sh",
		URL:    "https://github.com/chubin/cheat.sheets/archive/refs/heads/master.tar.gz",
		Format: FormatTarGz,
		Credit: "Special thanks to Igor Chubin and cheat.sh contributors! (https://github.com/chubin/cheat.sheets)",
	},
	{
		ID:     "cheat",
		Name:   "cheat",
		URL:    "https://github.com/cheat/cheatsheets/archive/refs/heads/master.tar.gz",
		Format: FormatTarGz,
		Credit: "Special thanks to Chris Allen Lane and cheat contributors! (https://github.com/cheat/cheatsheets)",
	},
}

// CheatsheetEntry holds a single discovered cheatsheet.
type CheatsheetEntry struct {
	Source  string // "tldr", "navi", "cheat-sh", "cheat"
	Name    string // command name e.g. "tar", "git-commit"
	Content string // text / markdown content
}

// DownloadAll fetches all configured cheatsheet sources and unpacks them under
// destBaseDir with path traversal checks and explicit attribution logging.
func DownloadAll(client *http.Client, destBaseDir string, stdout io.Writer) error {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	if err := os.MkdirAll(destBaseDir, 0755); err != nil {
		return fmt.Errorf("cannot create cheatsheet directory %s: %w", destBaseDir, err)
	}

	for idx, src := range DefaultSources {
		fmt.Fprintf(stdout, "[%d/%d] Downloading cheatsheets from %s...\n", idx+1, len(DefaultSources), src.Name)
		fmt.Fprintf(stdout, "  %s\n", src.Credit)

		data, err := fetchWithRedirects(client, src.URL)
		if err != nil {
			fmt.Fprintf(stdout, "  Warning: could not download cheatsheets for %s: %v\n", src.Name, err)
			continue
		}

		stagingDir := filepath.Join(destBaseDir, src.ID+"-staging.kk")
		_ = os.RemoveAll(stagingDir)
		if err := os.MkdirAll(stagingDir, 0755); err != nil {
			fmt.Fprintf(stdout, "  Warning: cannot create staging directory for %s: %v\n", src.Name, err)
			continue
		}

		var extractErr error
		switch src.Format {
		case FormatZip:
			extractErr = extractZip(data, stagingDir)
		case FormatTarGz:
			extractErr = extractTarGz(data, stagingDir)
		}

		if extractErr != nil {
			_ = os.RemoveAll(stagingDir)
			fmt.Fprintf(stdout, "  Warning: could not extract %s cheatsheets: %v\n", src.Name, extractErr)
			continue
		}

		targetDir := filepath.Join(destBaseDir, src.ID)
		_ = os.RemoveAll(targetDir)
		if err := os.Rename(stagingDir, targetDir); err != nil {
			// Fallback if rename fails across partitions
			_ = os.RemoveAll(stagingDir)
			fmt.Fprintf(stdout, "  Warning: could not install %s cheatsheets: %v\n", src.Name, err)
			continue
		}

		fmt.Fprintf(stdout, "  Extracted %d KB successfully.\n", len(data)/1024)
	}

	return nil
}

func fetchWithRedirects(client *http.Client, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "yups-installer")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	return io.ReadAll(resp.Body)
}

func extractZip(data []byte, destDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	cleanDest := filepath.Clean(destDir)
	for _, f := range reader.File {
		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(f.Name) {
			return fmt.Errorf("illegal path traversal in zip: %s", f.Name)
		}

		targetPath := filepath.Join(cleanDest, cleanName)
		if !strings.HasPrefix(targetPath, cleanDest+string(filepath.Separator)) && targetPath != cleanDest {
			return fmt.Errorf("illegal path traversal in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		_, copyErr := io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if copyErr != nil {
			return copyErr
		}
	}

	return nil
}

func extractTarGz(data []byte, destDir string) error {
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	cleanDest := filepath.Clean(destDir)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(header.Name) {
			return fmt.Errorf("illegal path traversal in tar: %s", header.Name)
		}

		targetPath := filepath.Join(cleanDest, cleanName)
		if !strings.HasPrefix(targetPath, cleanDest+string(filepath.Separator)) && targetPath != cleanDest {
			return fmt.Errorf("illegal path traversal in tar: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}

const maxCheatsheetBytes = 2048

// FindCheatsheets searches destBaseDir for any cheatsheet matching cmd and optional subcmd.
func FindCheatsheets(baseDir, cmd, subcmd string) []CheatsheetEntry {
	if baseDir == "" || cmd == "" {
		return nil
	}

	fi, err := os.Stat(baseDir)
	if err != nil || !fi.IsDir() {
		return nil
	}

	candidates := make(map[string]bool)
	cleanCmd := strings.ToLower(filepath.Base(cmd))
	candidates[cleanCmd] = true
	candidates[cleanCmd+".md"] = true
	candidates[cleanCmd+".cheat"] = true
	candidates[cleanCmd+".txt"] = true

	if subcmd != "" {
		cleanSub := strings.ToLower(subcmd)
		joinedHyphen := cleanCmd + "-" + cleanSub
		joinedUnder := cleanCmd + "_" + cleanSub
		candidates[joinedHyphen] = true
		candidates[joinedHyphen+".md"] = true
		candidates[joinedHyphen+".cheat"] = true
		candidates[joinedUnder] = true
		candidates[joinedUnder+".md"] = true
		candidates[joinedUnder+".cheat"] = true
		candidates[cleanSub] = true
		candidates[cleanSub+".md"] = true
		candidates[cleanSub+".cheat"] = true
	}

	var results []CheatsheetEntry
	seenPaths := make(map[string]bool)

	_ = filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		name := strings.ToLower(d.Name())
		if candidates[name] {
			if seenPaths[path] {
				return nil
			}
			seenPaths[path] = true

			contentBytes, err := os.ReadFile(path)
			if err != nil || len(contentBytes) == 0 {
				return nil
			}

			// Truncate to max bytes if necessary to prevent token overflow
			text := string(contentBytes)
			if len(text) > maxCheatsheetBytes {
				text = text[:maxCheatsheetBytes] + "\n... (truncated)"
			}

			sourceName := detectSource(baseDir, path)
			results = append(results, CheatsheetEntry{
				Source:  sourceName,
				Name:    name,
				Content: strings.TrimSpace(text),
			})
		}
		return nil
	})

	return results
}

func detectSource(baseDir, fullPath string) string {
	rel, err := filepath.Rel(baseDir, fullPath)
	if err != nil {
		return "community"
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) > 0 {
		return parts[0]
	}
	return "community"
}
