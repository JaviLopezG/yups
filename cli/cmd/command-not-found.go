package cmd

import (
	"log/slog"
	"strings"
)

var cnfMode bool

func init() {
	rootCmd.Flags().BoolVar(&cnfMode, "command-not-found",
		false, "System hook for command not found.")
	rootCmd.Flags().MarkHidden("command-not-found")
}

func handleCNF(args []string) {
	slog.Info("Straw-boss (CNF Mode) analyzing: ",
		"query", strings.Join(args, " "))
	//TODO identify the command and make suggestions.
}
