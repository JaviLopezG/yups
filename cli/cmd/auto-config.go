package cmd

import (
	"log/slog"
)

var acMode bool

func init() {
	rootCmd.Flags().BoolVar(&acMode, "auto-config",
		false, "Set configuration to default values.")
}

func handleAC() {
	slog.Info("Straw-boss (AC Mode).")
	//TODO identify the command and make suggestions.
}
