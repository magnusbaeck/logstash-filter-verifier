package app

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

const (
	exitCodeNormal = 0
	exitCodeError  = 1
)

func Execute(version string, stdout, stderr io.Writer) int {
	log := logging.MustGetLogger()
	viper.Set("logger", log)

	// Initialize config
	viper.SetConfigName("logstash-filter-verifier") // name of config file (without extension)
	viper.AddConfigPath(".")
	configDir, err := os.UserConfigDir()
	if err == nil {
		viper.AddConfigPath(path.Join(configDir, "logstash-filter-verifier"))
	}
	viper.AddConfigPath("/etc/logstash-filter-verifier/")

	// Setup default values
	viper.SetDefault("loglevel", "WARNING")
	viper.SetDefault("socket", "/tmp/logstash-filter-verifier.sock")
	viper.SetDefault("pipeline", "/etc/logstash/pipelines.yml")
	viper.SetDefault("logstash.path", "")

	// Read config
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Errorf("Error processing config file: %v", err)
			return exitCodeError
		}
	}

	rootCmd := makeRootCmd(version)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SilenceUsage = true

	if err := rootCmd.Execute(); err != nil {
		prefixedUserError(stderr, "error: %v", err)
		return exitCodeError
	}

	return exitCodeNormal
}

func makeRootCmd(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use: "logstash-filter-verifier",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			logging.SetLevel(viper.GetString("loglevel"))
		},
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
		SilenceErrors: true,
		Version:       version,
	}

	rootCmd.InitDefaultVersionFlag()

	rootCmd.PersistentFlags().String("loglevel", "WARNING", "Set the desired level of logging (one of: CRITICAL, ERROR, WARNING, NOTICE, INFO, DEBUG).")
	_ = viper.BindPFlag("loglevel", rootCmd.PersistentFlags().Lookup("loglevel"))

	rootCmd.AddCommand(makeStandaloneCmd())
	rootCmd.AddCommand(makeDaemonCmd())
	rootCmd.AddCommand(makeSetupCmd())

	return rootCmd
}

// prefixedUserError prints an error message to stderr and prefixes it
// with the name of the program file (e.g. "logstash-filter-verifier:
// something bad happened.").
func prefixedUserError(out io.Writer, format string, a ...interface{}) {
	basename := filepath.Base(os.Args[0])
	message := fmt.Sprintf(format, a...)
	if strings.HasSuffix(message, "\n") {
		fmt.Fprintf(out, "%s: %s", basename, message)
	} else {
		fmt.Fprintf(out, "%s: %s\n", basename, message)
	}
}
