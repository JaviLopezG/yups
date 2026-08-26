package semver

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input      string
		wantErr    bool
		isDev      bool
		major, min int
		patch      int
		preRelease string
	}{
		{input: "v1.2.3", wantErr: false, major: 1, min: 2, patch: 3},
		{input: "1.2.3", wantErr: false, major: 1, min: 2, patch: 3},
		{input: "v0.6.4", wantErr: false, major: 0, min: 6, patch: 4},
		{input: "v1.0.0-rc.1", wantErr: false, major: 1, min: 0, patch: 0, preRelease: "rc.1"},
		{input: "dev", wantErr: false, isDev: true},
		{input: "", wantErr: true},
		{input: "v1.2", wantErr: true},
		{input: "invalid", wantErr: true},
		{input: "v1.2.a", wantErr: true},
	}

	for _, tt := range tests {
		v, err := Parse(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("Parse(%q) err = %v, wantErr = %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr {
			if v.IsDev != tt.isDev {
				t.Errorf("Parse(%q).IsDev = %v, want %v", tt.input, v.IsDev, tt.isDev)
			}
			if !v.IsDev {
				if v.Major != tt.major || v.Minor != tt.min || v.Patch != tt.patch || v.PreRelease != tt.preRelease {
					t.Errorf("Parse(%q) = %+v, want %d.%d.%d-%s", tt.input, v, tt.major, tt.min, tt.patch, tt.preRelease)
				}
			}
		}
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		v1, v2 string
		want   int
	}{
		{"v1.0.0", "v1.0.0", 0},
		{"v1.0.0", "1.0.0", 0},
		{"v1.0.1", "v1.0.0", 1},
		{"v1.0.0", "v1.0.1", -1},
		{"v1.1.0", "v1.0.9", 1},
		{"v2.0.0", "v1.9.9", 1},
		{"v0.6.4", "v0.6.3", 1},
		{"v0.6.3", "v0.6.4", -1},
		{"dev", "v1.0.0", 1},
		{"v1.0.0", "dev", -1},
		{"dev", "dev", 0},
		{"v1.0.0-rc.1", "v1.0.0", -1},
		{"v1.0.0", "v1.0.0-rc.1", 1},
	}

	for _, tt := range tests {
		got := Compare(tt.v1, tt.v2)
		if got != tt.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
		}
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("v1.0.0", "v1.0.1") {
		t.Error("v1.0.1 should be newer than v1.0.0")
	}
	if IsNewer("v1.0.1", "v1.0.0") {
		t.Error("v1.0.0 should not be newer than v1.0.1")
	}
	if IsNewer("v1.0.0", "v1.0.0") {
		t.Error("v1.0.0 should not be newer than v1.0.0")
	}
	if !IsNewer("v0.1.0", "dev") {
		t.Error("dev should be newer than v0.1.0")
	}
	if IsNewer("dev", "v0.1.0") {
		t.Error("v0.1.0 should not be newer than dev")
	}
}
