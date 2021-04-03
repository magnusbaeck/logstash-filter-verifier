package app

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/mod/semver"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/app/setup"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

func makeSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup [<flags>] <logstash-version>",
		Short: "Setup the given version of Logstash for usage with logstash-filter-verifier",
		RunE:  runSetup,
		Args:  validateSetupArgs,
	}

	cmd.Flags().StringP("target-dir", "t", "./3rdparty", "target directory, where Logstash is downloaded and unarchived to")
	_ = viper.BindPFlag("target-dir", cmd.Flags().Lookup("target-dir"))

	cmd.Flags().Bool("oss", false, "setup oss release of Logstash")
	_ = viper.BindPFlag("oss", cmd.Flags().Lookup("oss"))

	cmd.Flags().String("os-arch", osArch(), "os and arch string for the Logstash version to be downloaded, e.g. linux-x86_64")
	_ = viper.BindPFlag("os-arch", cmd.Flags().Lookup("os-arch"))

	cmd.Flags().String("archive-type", "tar.gz", "archive type to be downloaded, e.g. tar.gz or zip")
	_ = viper.BindPFlag("archive-type", cmd.Flags().Lookup("archive-type"))

	return cmd
}

func runSetup(_ *cobra.Command, args []string) error {
	s := setup.New(
		args[0],
		viper.GetString("target-dir"),
		viper.GetBool("oss"),
		viper.GetString("os-arch"),
		viper.GetString("archive-type"),
		viper.Get("logger").(logging.Logger),
	)

	return s.Run()
}

func validateSetupArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("required argument 'logstash-version' not provided, try --help")
	}

	if !semver.IsValid("v" + args[0]) {
		return errors.New("invalid version string provided, correct format x.y.z, e.g. 7.12.0")
	}
	return nil
}

func osArch() string {
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	}

	return fmt.Sprintf("%s-%s", runtime.GOOS, arch)
}
