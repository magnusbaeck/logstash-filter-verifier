package app

import (
	"context"
	"time"

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

	cmd.Flags().String("logstash-path", "/usr/share/logstash/bin/logstash", "location where the logstash executable is found")
	_ = viper.BindPFlag("logstash.path", cmd.Flags().Lookup("logstash-path"))

	cmd.Flags().Duration("inflight-shutdown-timeout", 10*time.Second, "maximum duration to wait for in-flight test executions to finish during shutdown")
	_ = viper.BindPFlag("inflight-shutdown-timeout", cmd.Flags().Lookup("inflight-shutdown-timeout"))

	cmd.Flags().Duration("shutdown-timeout", 3*time.Second, "maximum duration to wait for Logstash and gRPC server to gracefully shutdown")
	_ = viper.BindPFlag("shutdown-timeout", cmd.Flags().Lookup("shutdown-timeout"))

	cmd.Flags().Duration("wait-for-state-timeout", 30*time.Second, "maximum duration to wait for Logstash to reach an expected state (e.g. pipeline ready)")
	_ = viper.BindPFlag("wait-for-state-timeout", cmd.Flags().Lookup("wait-for-state-timeout"))

	cmd.Flags().Bool("no-cleanup", false, "with no-cleanup set to true, the temporary files (logstash pipelines, logs, etc.) are not cleaned up and left on the system for later inspection.")
	_ = viper.BindPFlag("no-cleanup", cmd.Flags().Lookup("no-cleanup"))

	// TODO: Move default values to some sort of global lookup like defaultKeptEnvVars.
	// TODO: Not yet sure, if this should be global or only in standalone.
	cmd.Flags().StringSlice("keep-env", nil, "Add this environment variable to the list of variables that will be preserved from the calling process's environment.")
	_ = viper.BindPFlag("daemon-keep-envs", cmd.Flags().Lookup("keep-env"))

	return cmd
}

func runDaemonStart(_ *cobra.Command, _ []string) error {
	socket := viper.GetString("socket")
	logstashPath := viper.GetString("logstash.path")
	keptEnvs := viper.GetStringSlice("keep-envs")
	if len(viper.GetStringSlice("daemon-keep-envs")) > 0 {
		keptEnvs = viper.GetStringSlice("daemon-keep-envs")
	}
	inflightShutdownTimeout := viper.GetDuration("inflight-shutdown-timeout")
	shutdownTimeout := viper.GetDuration("shutdown-timeout")
	waitForStateTimeout := viper.GetDuration("wait-for-state-timeout")
	noCleanup := viper.GetBool("no-cleanup")
	log := viper.Get("logger").(logging.Logger)

	log.Debugf("config: socket: %s", socket)
	log.Debugf("config: logstash-path: %s", logstashPath)

	s := daemon.New(socket, logstashPath, keptEnvs, log, inflightShutdownTimeout, shutdownTimeout, waitForStateTimeout, noCleanup)
	defer s.Cleanup()

	return s.Run(context.Background())
}
