package app

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/app/daemon/run"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

func makeDaemonRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run test suite with logstash-filter-verifier daemon",
		RunE:  runDaemonRun,
	}

	cmd.Flags().StringP("pipeline", "p", "", "location of the pipelines.yml file to be processed")
	_ = viper.BindPFlag("pipeline", cmd.Flags().Lookup("pipeline"))
	cmd.Flags().StringP("pipeline-base", "", "", "base directory for relative paths in the pipelines.yml")
	_ = viper.BindPFlag("pipeline-base", cmd.Flags().Lookup("pipeline-base"))
	cmd.Flags().StringP("testcase-dir", "t", "", "directory containing the test case files")
	_ = viper.BindPFlag("testcase-dir", cmd.Flags().Lookup("testcase-dir"))
	cmd.Flags().Bool("debug", false, "enable debug mode; e.g. prevents stripping '__lfv' data from Logstash events")
	_ = viper.BindPFlag("debug", cmd.Flags().Lookup("debug"))

	return cmd
}

func runDaemonRun(_ *cobra.Command, args []string) error {
	socket := viper.GetString("socket")
	log := viper.Get("logger").(logging.Logger)
	pipeline := viper.GetString("pipeline")
	pipelineBase := viper.GetString("pipeline-base")
	testcaseDir := viper.GetString("testcase-dir")
	debug := viper.GetBool("debug")

	t, err := run.New(socket, log, pipeline, pipelineBase, testcaseDir, debug)
	if err != nil {
		return err
	}

	return t.Run()
}
