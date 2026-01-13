package main

import (
	"log/slog"

	"github.com/javilopezg/yups/cli/cmd"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			slog.Error("Yups panicked.", "Error", err)
		}
	}()
	cmd.Execute()
}
