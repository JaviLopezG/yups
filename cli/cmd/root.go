package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/javilopezg/yups/cli/internal/sys"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	debug   bool
)

var rootCmd = &cobra.Command{
	Use:   "yups",
	Short: "YUPS: Your Universal Prompt Straw-boss (AI Powered)",
	Long: `The YUPS CLI handles your command not found and other
prompt errors. It can solve any situation or requirement 
by querying an online LLM.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		setupLogger(debug)

		return checkConfig()
	},
	Run: func(cmd *cobra.Command, args []string) {
		if cnfMode {
			handleCNF(args)
			return
		}
		if ceMode {
			handleCE(args)
			return
		}
		if acMode {
			handleAC()
			return
		}
		if arMode {
			handleAR()
			return
		}
		if len(args) == 0 {
			cmd.Help()
			return
		}
		processQuery(args)
		return
	},
}

func checkConfig() error {
	err := viper.ReadInConfig()
	if err == nil {
		return nil
	}

	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		slog.Warn("Configuration file not found.")

		answer, _ := sys.PromptConfirmReplacement("yups --auto-config")

		if answer {
			return nil
		}
	}

	return sys.YupsError{
		"Yups needs to be configured before execution. Try 'yups --auto-config'.",
		sys.ExitUsage, err,
	}
}

func processQuery(args []string) {
	//TODO process user query
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		yErr, ok := err.(sys.YupsError)
		if !ok {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println(yErr.Message)
		os.Exit(yErr.Code)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().
		StringVar(&cfgFile, "config", "",
			"Configuration file (default: $HOME/.yups/config.toml)")
	rootCmd.PersistentFlags().
		BoolVarP(&debug, "debug", "d",
			false, "set the log level to debug")

	viper.BindPFlag("debug",
		rootCmd.PersistentFlags().Lookup("debug"))

}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			slog.Error("Error getting home directory.", "Error", err)
			os.Exit(1)
		}

		viper.AddConfigPath(home + "/.yups")
		viper.SetConfigType("toml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv()

	viper.ReadInConfig()
	slog.Debug("Setting config file.", "ConfigFileUsed", viper.ConfigFileUsed())
}

func setupLogger(isDebug bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Error("Error getting home directory.", "Error", err)
		os.Exit(1)
	}
	folder := filepath.Join(home, ".yups")
	os.MkdirAll(folder, 0755)
	logFile, err := os.OpenFile(folder+"/log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	level := slog.LevelInfo

	if isDebug {
		level = slog.LevelDebug
	}
	handler := sys.NewYupsHandler(logFile, level)
	logger := slog.New(handler)
	slog.SetDefault(logger)
	if err != nil {
		slog.Error("Error setting file log", "Error", err)
	}
}
