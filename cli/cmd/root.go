package cmd

import (
	"fmt"
	"log/slog"
	"os"

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
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogger(debug)
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
		if len(args) == 0 {
			cmd.Help()
			return
		}
		processQuery(args)
		return
	},
}

func processQuery(args []string) {
	//TODO process user query
}

func Execute() {
	slog.Debug("Executing yups")
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
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

	err := viper.ReadInConfig()
	slog.Debug("Setting config file.", "ConfigFileUsed", viper.ConfigFileUsed())

	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		handleAC()
	}
}

func setupLogger(isDebug bool) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	if isDebug {
		opts.Level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, opts))
	slog.SetDefault(logger)
}
