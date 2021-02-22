package app

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/app/daemon/run"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

func makeDaemonRunCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "run",
		Short: "Run test suite with logstash-filter-verifier daemon",
		RunE:  runDaemonRun,
	}

	rootCmd.Flags().StringP("pipeline", "p", "", "location of the pipelines.yml file to be processed")
	_ = viper.BindPFlag("pipeline", rootCmd.Flags().Lookup("pipeline"))
	rootCmd.Flags().StringP("pipeline-base", "", "", "base directory for relative paths in the pipelines.yml")
	_ = viper.BindPFlag("pipeline-base", rootCmd.Flags().Lookup("pipeline-base"))
	rootCmd.Flags().StringP("testcase-dir", "t", "", "directory containing the test case files")
	_ = viper.BindPFlag("testcase-dir", rootCmd.Flags().Lookup("testcase-dir"))

	return rootCmd
}

func runDaemonRun(_ *cobra.Command, args []string) error {
	// TODO: Remove this
	// logging.SetLevel(oplogging.INFO)

	t, err := run.New(viper.GetString("socket"), viper.Get("logger").(logging.Logger), viper.GetString("pipeline"), viper.GetString("pipeline-base"), viper.GetString("testcase-dir"))
	if err != nil {
		return err
	}

	return t.Run()
}
