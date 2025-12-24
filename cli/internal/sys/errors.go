package sys

import "fmt"

const (
	ExitOK          = 0
	ExitUsage       = 64 // Error de configuración o parámetros
	ExitNoPerm      = 77 // Error de permisos
	ExitNotFound    = 127
	ExitUserCtrlC   = 130
	ExitFailure     = 1
	ExitUnavailable = 69
)

type YupsError struct {
	Message string
	Code    int
	Err     error
}

func (e *YupsError) Error() string {
	return fmt.Sprintf("Yups error (code %d): %s \n Inner: %v", e.Code, e.Message, e.Err)
}

func (e *YupsError) Unwrap() error { return e.Err }
