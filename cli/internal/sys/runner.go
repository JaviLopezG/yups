package sys

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

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

func PromptConfirmReplacement(command string) (bool, error) {
	if !isInteractive() {
		return false, nil
	}

	Prompt("Suggestion: " + command)

	var response string
	_, err := fmt.Scanln(&response)
	if err != nil && err.Error() != "unexpected newline" {
		return false, err
	}

	response = strings.ToLower(strings.TrimSpace(response))
	if response == "" || response == "y" || response == "yes" {
		slog.Info("User accepted command execution", "command", command)

		subs := strings.Split(command, " ")
		return true, actualRunnerReplacement(subs[0], subs[1:]...)
	}
	slog.Info("User rejected command execution", "command", command)
	return false, nil
}

func actualRunnerReplacement(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
