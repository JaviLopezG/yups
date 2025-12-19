package cmd

import (
	"log/slog"
	"strings"
)

var ceMode bool

func init() {
	rootCmd.Flags().BoolVar(&ceMode, "command-error",
		false, "System hook for command error.")
	rootCmd.Flags().MarkHidden("command-error")
}

func handleCE(args []string) {
	slog.Info("Straw-boss (CE Mode) analyzing: ",
		"query", strings.Join(args, " "))
	//TODO identify the command and make suggestions.
}
