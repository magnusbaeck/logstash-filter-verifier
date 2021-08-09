package app

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/app/standalone"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

func makeStandaloneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "standalone [<flags>] <testcases> <config>...",
		Short: "Run logstash-filter-verifier in standalone mode",
		RunE:  runStandalone,
		Args:  validateStandaloneArgs,
	}

	// TODO: This flag makes sense with daemon mode test command as well.
	cmd.Flags().String("diff-command", "diff -u", "Set the command to run to compare two events. The command will receive the two files to compare as arguments.")
	_ = viper.BindPFlag("diff-command", cmd.Flags().Lookup("diff-command"))

	// TODO: Move default values to some sort of global lookup like defaultKeptEnvVars.
	// TODO: Not yet sure, if this should be global or only in standalone.
	cmd.Flags().StringSlice("keep-env", nil, "Add this environment variable to the list of variables that will be preserved from the calling process's environment.")
	_ = viper.BindPFlag("keep-envs", cmd.Flags().Lookup("keep-env"))

	// TODO: Not yet sure, if this should be global or only in standalone.
	cmd.Flags().StringSlice("logstash-arg", nil, "Command line arguments, which are passed to Logstash. Flag and value have to be provided as a flag each, e.g.: --logstash-arg=-n --logstash-arg=InstanceName")
	_ = viper.BindPFlag("logstash-args", cmd.Flags().Lookup("logstash-arg"))

	// TODO: Not yet sure, if this should be global or only in standalone.
	cmd.Flags().Bool("logstash-output", false, "Print the debug output of logstash.")
	_ = viper.BindPFlag("logstash-output", cmd.Flags().Lookup("logstash-output"))

	// TODO: Not yet sure, if this should be global or only in standalone.
	cmd.Flags().StringSlice("logstash-path", nil, "Add a path to the list of Logstash executable paths that will be tried in order (first match is used).")
	_ = viper.BindPFlag("logstash-paths", cmd.Flags().Lookup("logstash-path"))

	// TODO: Not yet sure, if this should be global or only in standalone.
	cmd.Flags().String("logstash-version", "auto", "The version of Logstash that's being targeted.")
	_ = viper.BindPFlag("logstash-version", cmd.Flags().Lookup("logstash-version"))

	cmd.Flags().Bool("sockets", false, "Use Unix domain sockets for the communication with Logstash.")
	_ = viper.BindPFlag("sockets", cmd.Flags().Lookup("sockets"))

	cmd.Flags().Duration("sockets-timeout", 60*time.Second, "Timeout (duration) for the communication with Logstash via Unix domain sockets. Has no effect unless --sockets is used.")
	_ = viper.BindPFlag("sockets-timeout", cmd.Flags().Lookup("sockets-timeout"))

	// TODO: Not yet sure, if this should be global or only in standalone.
	cmd.Flags().Bool("quiet", false, "Omit test progress messages and event diffs.")
	_ = viper.BindPFlag("quiet", cmd.Flags().Lookup("quiet"))

	return cmd
}

func runStandalone(_ *cobra.Command, args []string) error {
	s := standalone.New(
		viper.GetBool("quiet"),
		viper.GetString("diff-command"),
		args[0],
		viper.GetStringSlice("keep-envs"),
		viper.GetStringSlice("logstash-paths"),
		viper.GetString("logstash-version"),
		viper.GetStringSlice("logstash-args"),
		viper.GetBool("logstash-output"),
		args[1:],
		viper.GetBool("sockets"),
		viper.GetDuration("sockets-timeout"),
		viper.Get("logger").(logging.Logger),
	)

	return s.Run()
}

func validateStandaloneArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("required argument 'testcases' not provided, try --help")
	}
	if len(args) < 2 {
		return errors.New("required argument 'config' not provided, try --help")
	}
	for _, arg := range args {
		_, err := os.Stat(arg)
		if os.IsNotExist(err) {
			return fmt.Errorf("path %q does not exist, try --help", arg)
		}
	}
	return nil
}
