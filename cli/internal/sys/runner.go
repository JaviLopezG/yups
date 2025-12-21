package sys

import (
	"bytes"
	"errors"
	"os"
	"os/exec"

	"golang.org/x/term"
)

var SudoRunner = actualSudoRunner
var Runner = actualRunner

func RunSudoCommand(name string, args ...string) error {
	return SudoRunner(name, args...)
}

func actualSudoRunner(name string, args ...string) error {
	isRoot := os.Geteuid() == 0
	isInt := isInteractive()
	theName := name
	theArgs := args

	if !isInt && !isRoot {
		return errors.New("non-interactive terminal: sudo requires a TTY or root privileges")
	}
	if !isRoot {
		theArgs = append([]string{name}, args...)
		theName = "sudo"
	}

	cmd := exec.Command(theName, theArgs...)
	if isInt {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func RunCommand(provides string, args ...string) (string, error) {
	return Runner(provides, args...)
}

func actualRunner(provides string, args ...string) (string, error) {
	cmd := exec.Command(provides, args...)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	err := cmd.Run()
	if err == nil && errb.String() != "" {
		err = errors.New(errb.String())
	}
	return outb.String(), err
}
