package sys

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSystemInfo(t *testing.T) {
	info := GetSystemInfo()
	assert.NotNil(t, info)
	assert.NotEmpty(t, info.OS)
	assert.NotEmpty(t, info.DistroID)
	assert.NotEmpty(t, info.DistroVersion)
	assert.NotEmpty(t, info.DistroPretty)
	assert.NotEmpty(t, info.PM)
}

func TestDetectManager(t *testing.T) {
	manager := detectPM()
	if manager == "" {
		t.Error("Manager should not be empty string")
	}
	assert.NotEqual(t, "unknown", manager)
	t.Logf("Detected manager: %s", manager)
}
