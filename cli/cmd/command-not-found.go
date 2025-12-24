package cmd

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/spf13/viper"
	"github.com/tu-usuario/yups/cli/internal/parser"
	"github.com/tu-usuario/yups/cli/internal/sys"
)

var cnfMode bool

func init() {
	rootCmd.Flags().BoolVar(&cnfMode, "command-not-found",
		false, "System hook for command not found.")
	rootCmd.Flags().MarkHidden("command-not-found")
}

func handleCNF(args []string) {
	query := strings.Join(args, " ")
	slog.Info("Straw-boss (CNF Mode) analyzing: ",
		"query", query)

	lastCommand := viper.GetString("YUPS_LAST_CMD")
	slog.Debug("Last command: ", lastCommand)
	command, _ := parser.ExtractEffectiveCommand(lastCommand)
	//TODO if command is in sys.PMTypes analyze and replace
	if slices.Contains(sys.PMTypes, command) {
		//TODO identify the subcommand
		//TODO identify the packages
		//TODO suggest correct command Y/n
		//TODO if not suggest yups query for deep analysis Y/n
		return
	}
	//TODO if command is similar to one in scanned suggest
	//TODO is command in currentCommands with fuzzy search
	//TODO suggest correct command Y/n
	//TODO if not suggest yups query for deep analysis Y/n
	//TODO execute provides, parse output and suggest install
	replacer := strings.NewReplacer(sys.ArgumentString, command)
	provides := replacer.Replace(
		sys.PMCommands["provides"].Commands[viper.GetString("pm")])
	output, err := sys.RunCommand(provides)
	if err == nil {
		//TODO parse output???
		slog.Debug("Provides output", "output", output)
	}
}
