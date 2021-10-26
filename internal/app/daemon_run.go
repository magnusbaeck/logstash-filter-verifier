package app

import (
	"github.com/pkg/errors"
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

	cmd.Flags().StringP("pipeline", "p", "", "location of the pipelines.yml file to be processed (e.g. /etc/logstash/pipelines.yml)")
	_ = viper.BindPFlag("pipeline", cmd.Flags().Lookup("pipeline"))
	cmd.Flags().String("pipeline-base", "", "base directory for relative paths in the pipelines.yml")
	_ = viper.BindPFlag("pipeline-base", cmd.Flags().Lookup("pipeline-base"))
	cmd.Flags().String("logstash-config", "", "path of the Logstash config for use, if no pipelines.yml exists (mutual exclusive with --pipeline flag).")
	_ = viper.BindPFlag("logstash-config", cmd.Flags().Lookup("logstash-config"))
	cmd.Flags().StringP("testcase-dir", "t", "", "directory containing the test case files")
	_ = viper.BindPFlag("testcase-dir", cmd.Flags().Lookup("testcase-dir"))
	cmd.Flags().String("plugin-mock", "", "path to a yaml file containing the definition for the plugin mocks.")
	_ = viper.BindPFlag("plugin-mock", cmd.Flags().Lookup("plugin-mock"))
	cmd.Flags().Bool("debug", false, "enable debug mode; e.g. prevents stripping '__lfv_' prefixed fields/tags from Logstash events")
	_ = viper.BindPFlag("debug", cmd.Flags().Lookup("debug"))
	cmd.Flags().String("metadata-key", "@metadata", "Key under which the content of the `@metadata` field is exposed in the returned events.")
	_ = viper.BindPFlag("metadata-key", cmd.Flags().Lookup("metadata-key"))
	cmd.Flags().Bool("add-missing-id", false, "add implicit id for the plugins in the Logstash config if they are missing")
	_ = viper.BindPFlag("add-missing-id", cmd.Flags().Lookup("add-missing-id"))

	return cmd
}

func runDaemonRun(_ *cobra.Command, args []string) error {
	socket := viper.GetString("socket")
	log := viper.Get("logger").(logging.Logger)
	pipeline := viper.GetString("pipeline")
	pipelineBase := viper.GetString("pipeline-base")
	logstashConfig := viper.GetString("logstash-config")
	testcaseDir := viper.GetString("testcase-dir")
	pluginMock := viper.GetString("plugin-mock")
	debug := viper.GetBool("debug")
	metadataKey := viper.GetString("metadata-key")
	addMissingID := viper.GetBool("add-missing-id")

	if pipeline != "" && logstashConfig != "" {
		return errors.New("--pipeline and --logstash-config flags are mutual exclusive")
	}

	t, err := run.New(socket, log, pipeline, pipelineBase, logstashConfig, testcaseDir, pluginMock, metadataKey, debug, addMissingID)
	if err != nil {
		return err
	}

	return t.Run()
}
