package app

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func makeDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Control logstash-filter-verifier daemon mode",
	}

	cmd.PersistentFlags().StringP("socket", "s", "", "location of the control socket")
	_ = viper.BindPFlag("socket", cmd.PersistentFlags().Lookup("socket"))

	cmd.AddCommand(makeDaemonStartCmd())
	cmd.AddCommand(makeDaemonShutdownCmd())
	cmd.AddCommand(makeDaemonRunCmd())

	return cmd
}
