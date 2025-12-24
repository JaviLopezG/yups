// internal/sys/errors_test.go
package sys

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestYupsError_Error(t *testing.T) {
	inner := errors.New("original error")
	e := &YupsError{
		Message: "something went wrong",
		Code:    ExitUsage,
		Inner:   inner,
	}

	out := e.Error()
	assert.Contains(t, out, "code 64")
	assert.Contains(t, out, "something went wrong")
	assert.Contains(t, out, "original error")
}
