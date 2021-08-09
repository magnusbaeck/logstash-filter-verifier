package app

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/app/daemon/shutdown"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

func makeDaemonShutdownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shutdown",
		Short: "Shutdown logstash-filter-verifier daemon",
		RunE:  runDaemonShutdown,
	}

	return cmd
}

func runDaemonShutdown(_ *cobra.Command, _ []string) error {
	s := shutdown.New(viper.GetString("socket"), viper.Get("logger").(logging.Logger))

	return s.Run()
}
