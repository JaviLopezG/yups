// Package update implements the self-update machinery: querying release
// sources (Forgejo canonical, GitHub fallback), downloading and verifying
// assets, extracting them and validating the new binary before it replaces
// the installed one.
package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
)

// ChecksumsFileName is the goreleaser-generated checksum asset shipped
// with every release.
const ChecksumsFileName = "checksums.txt"

// Release describes the latest published release, narrowed down to what
// the self-update needs: the tag and the download URLs of the archive for
// the running platform and of the checksum list.
type Release struct {
	Tag         string
	AssetURL    string
	ChecksumURL string
}

// ReleaseSource queries a repository host for its latest release.
type ReleaseSource interface {
	Latest() (Release, error)
}

// NewSource returns the release source matching the repository URL host:
// github.com talks to api.github.com; every other host is treated as a
// Gitea/Forgejo family server exposing its API under /api/v1 of its own
// base URL (approved design decision 5).
func NewSource(repoURL string, client *http.Client) (ReleaseSource, error) {
	api, err := apiLatestURL(repoURL)
	if err != nil {
		return nil, err
	}
	return &apiSource{apiURL: api, client: client}, nil
}

// apiLatestURL derives the releases/latest API endpoint from a repository
// web URL of the form scheme://host/owner/repo.
func apiLatestURL(repoURL string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("parsing repository URL %q: %w", repoURL, err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("repository URL %q must be http(s)", repoURL)
	}

	var segments []string
	for _, s := range strings.Split(u.Path, "/") {
		if s != "" {
			segments = append(segments, s)
		}
	}
	if len(segments) != 2 {
		return "", fmt.Errorf("repository URL %q must point to <host>/<owner>/<repo>", repoURL)
	}

	repoPath := segments[0] + "/" + segments[1]
	if u.Host == "github.com" {
		return "https://api.github.com/repos/" + repoPath + "/releases/latest", nil
	}
	return u.Scheme + "://" + u.Host + "/api/v1/repos/" + repoPath + "/releases/latest", nil
}

// apiSource queries one already-derived API endpoint. Both GitHub and the
// Gitea/Forgejo family answer releases/latest with compatible JSON shapes,
// so a single parser serves both.
type apiSource struct {
	apiURL string
	client *http.Client
}

// ArchiveName returns the goreleaser archive file name of a release for
// the running platform: yups-<tag>-<GOOS>-<GOARCH>.tar.gz.
func ArchiveName(tag string) string {
	return fmt.Sprintf("yups-%s-%s-%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)
}

// Latest fetches and parses the latest release, selecting the archive
// asset built for the running GOOS/GOARCH.
func (s *apiSource) Latest() (Release, error) {
	data, err := Fetch(s.client, s.apiURL)
	if err != nil {
		return Release{}, fmt.Errorf("querying latest release: %w", err)
	}
	return parseRelease(data)
}

// parseRelease extracts the tag and the platform asset URLs from a
// releases/latest JSON document.
func parseRelease(data []byte) (Release, error) {
	var payload struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return Release{}, fmt.Errorf("malformed release document: %w", err)
	}
	if payload.TagName == "" {
		return Release{}, fmt.Errorf("release document has no tag_name")
	}

	release := Release{Tag: payload.TagName}
	wantArchive := ArchiveName(payload.TagName)
	for _, asset := range payload.Assets {
		switch asset.Name {
		case wantArchive:
			release.AssetURL = asset.URL
		case ChecksumsFileName:
			release.ChecksumURL = asset.URL
		}
	}
	if release.AssetURL == "" {
		return Release{}, fmt.Errorf("release %s ships no %s asset", payload.TagName, wantArchive)
	}
	if release.ChecksumURL == "" {
		return Release{}, fmt.Errorf("release %s ships no %s asset", payload.TagName, ChecksumsFileName)
	}
	return release, nil
}
