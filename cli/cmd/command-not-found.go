package cmd

import (
	"bytes"
	"log/slog"
	"os/exec"
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
	commands, _ := parser.ExtractCommands(lastCommand)
	replacer := strings.NewReplacer(sys.PackagesString, commands[0])
	provides := replacer.Replace(
		sys.PMCommands["provides"].Commands[viper.GetString("pm")])
	cmd := exec.Command(provides)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	err := cmd.Run()
	if err == nil {
		//TODO parse output
		slog.Debug("Provides output", "output", string(outb.Bytes()))
	}
}
