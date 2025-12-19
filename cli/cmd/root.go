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
}

func Execute() {
	if debug {
		fmt.Println("Executing root command...")
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().
		StringVar(&cfgFile, "config", "",
			"configuration file (default: $HOME/.yups/config.toml)")
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
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(home + "/.yups")
		viper.SetConfigType("toml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil && debug {
		fmt.Println("📄 Setting config file:", viper.ConfigFileUsed())
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
