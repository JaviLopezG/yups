package update

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestAPILatestURLDerivation(t *testing.T) {
	tests := []struct {
		name    string
		repoURL string
		want    string
		wantErr bool
	}{
		{
			name:    "github host uses api.github.com",
			repoURL: "https://github.com/JaviLopezG/yups",
			want:    "https://api.github.com/repos/JaviLopezG/yups/releases/latest",
		},
		{
			name:    "forgejo host keeps its own base url",
			repoURL: "https://code.javilopezg.com/javilopezg/yups",
			want:    "https://code.javilopezg.com/api/v1/repos/javilopezg/yups/releases/latest",
		},
		{
			name:    "trailing slash is tolerated",
			repoURL: "https://code.javilopezg.com/javilopezg/yups/",
			want:    "https://code.javilopezg.com/api/v1/repos/javilopezg/yups/releases/latest",
		},
		{
			name:    "http scheme is kept for tests and local instances",
			repoURL: "http://forgejo.local:3000/own/rep",
			want:    "http://forgejo.local:3000/api/v1/repos/own/rep/releases/latest",
		},
		{name: "missing repo segment errors", repoURL: "https://github.com/JaviLopezG", wantErr: true},
		{name: "extra path segments error", repoURL: "https://github.com/a/b/c", wantErr: true},
		{name: "non http scheme errors", repoURL: "ftp://github.com/a/b", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := apiLatestURL(tt.repoURL)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("apiLatestURL(%q) = %q, want error", tt.repoURL, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("apiLatestURL(%q): %v", tt.repoURL, err)
			}
			if got != tt.want {
				t.Errorf("apiLatestURL(%q) = %q, want %q", tt.repoURL, got, tt.want)
			}
		})
	}
}

// releaseJSON builds a releases/latest document with the platform archive
// and the checksum list as assets.
func releaseJSON(tag string) string {
	archive := fmt.Sprintf("yups_%s_%s_%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)
	return fmt.Sprintf(`{
		"tag_name": %q,
		"assets": [
			{"name": %q, "browser_download_url": "https://dl.example/%s"},
			{"name": %q, "browser_download_url": "https://dl.example/%s"}
		]
	}`, tag, archive, archive, ChecksumsFileName, ChecksumsFileName)
}

func TestLatestQueriesForgejoAPIPathAndParsesRelease(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprint(w, releaseJSON("v1.2.3"))
	}))
	defer server.Close()

	source, err := NewSource(server.URL+"/owner/repo", server.Client())
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	release, err := source.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}

	wantPath := "/api/v1/repos/owner/repo/releases/latest"
	if gotPath != wantPath {
		t.Errorf("queried path = %q, want %q", gotPath, wantPath)
	}
	wantArchive := fmt.Sprintf("yups_v1.2.3_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	switch {
	case release.Tag != "v1.2.3":
		t.Errorf("Tag = %q, want v1.2.3", release.Tag)
	case release.AssetURL != "https://dl.example/"+wantArchive:
		t.Errorf("AssetURL = %q, want the %s asset", release.AssetURL, wantArchive)
	case release.ChecksumURL != "https://dl.example/"+ChecksumsFileName:
		t.Errorf("ChecksumURL = %q, want the %s asset", release.ChecksumURL, ChecksumsFileName)
	}
}

func TestLatestParsesGitHubShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, releaseJSON("v0.1.0"))
	}))
	defer server.Close()

	// The GitHub shape is served by api.github.com; point an apiSource at
	// a local double to exercise the parser against it.
	source := &apiSource{apiURL: server.URL + "/repos/owner/repo/releases/latest", client: server.Client()}
	release, err := source.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if release.Tag != "v0.1.0" {
		t.Errorf("Tag = %q, want v0.1.0", release.Tag)
	}
}

func TestLatestErrorPaths(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr string
	}{
		{
			name: "not found surfaces the status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "not found", http.StatusNotFound)
			},
			wantErr: "404",
		},
		{
			name: "malformed json is rejected",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "{definitely not json")
			},
			wantErr: "malformed release document",
		},
		{
			name: "missing tag name is rejected",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"assets": []}`)
			},
			wantErr: "no tag_name",
		},
		{
			name: "missing platform asset is rejected",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"tag_name": "v9.9.9", "assets": [
					{"name": "checksums.txt", "browser_download_url": "https://dl.example/checksums.txt"}]}`)
			},
			wantErr: "ships no yups_v9.9.9_",
		},
		{
			name: "missing checksums asset is rejected",
			handler: func(w http.ResponseWriter, r *http.Request) {
				archive := fmt.Sprintf("yups_v9.9.9_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
				fmt.Fprintf(w, `{"tag_name": "v9.9.9", "assets": [
					{"name": %q, "browser_download_url": "https://dl.example/x"}]}`, archive)
			},
			wantErr: "ships no " + ChecksumsFileName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			source := &apiSource{apiURL: server.URL, client: server.Client()}
			_, err := source.Latest()
			if err == nil {
				t.Fatal("Latest: want error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}
