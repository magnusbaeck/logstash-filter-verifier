package app

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/app/daemon"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

func makeDaemonStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start logstash-filter-verifier daemon",
		RunE:  runDaemonStart,
	}

	cmd.Flags().StringP("logstash-path", "", "/usr/share/logstash/bin/logstash", "location where the logstash executable is found")
	_ = viper.BindPFlag("logstash.path", cmd.Flags().Lookup("logstash-path"))

	return cmd
}

func runDaemonStart(_ *cobra.Command, _ []string) error {
	socket := viper.GetString("socket")
	logstashPath := viper.GetString("logstash.path")
	log := viper.Get("logger").(logging.Logger)

	log.Debugf("config: socket: %s", socket)
	log.Debugf("config: logstash-path: %s", logstashPath)

	s := daemon.New(socket, logstashPath, log)
	defer s.Cleanup()

	return s.Run(context.Background())
}
