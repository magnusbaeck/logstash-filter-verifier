package setup

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v3"
	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	"gopkg.in/cheggaaa/pb.v2"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

type Setup struct {
	version     string
	targetDir   string
	oss         bool
	osArch      string
	archiveType string
	log         logging.Logger
}

// New creates a new setup command to install versions of Logstash.
func New(version string, targetDir string, oss bool, osArch string, archiveType string, log logging.Logger) Setup {
	return Setup{
		version:     version,
		targetDir:   targetDir,
		oss:         oss,
		osArch:      osArch,
		archiveType: archiveType,
		log:         log,
	}
}

const firstOSSpecificRelease = "v7.10.0"

// Setup installs versions of Logstash.
func (s *Setup) Run() error {
	downloadDir := filepath.Join(s.targetDir, "/downloads")

	err := os.MkdirAll(downloadDir, 0775)
	if err != nil {
		return errors.Wrap(err, "failed to create base directory for Logstash setup")
	}

	oss := ""
	if s.oss {
		oss = "oss-"
	}
	filename := fmt.Sprintf("logstash-%s%s-%s.%s", oss, s.version, s.osArch, s.archiveType)
	targetDirVersion := path.Join(s.targetDir, fmt.Sprintf("logstash-%s%s-%s", oss, s.version, s.osArch))
	if semver.Compare("v"+s.version, firstOSSpecificRelease) < 0 {
		filename = fmt.Sprintf("logstash-%s%s.%s", oss, s.version, s.archiveType)
		targetDirVersion = path.Join(s.targetDir, fmt.Sprintf("logstash-%s%s-%s", oss, s.version, s.osArch))
	}

	targetFile := filepath.Join(downloadDir, filename)
	if !fileExists(targetFile) {
		s.log.Infof("Download of Logstash version %s (%s)", s.version, s.osArch)

		err = download(filename, targetFile)
		if err != nil {
			return errors.Wrap(err, "failed to download archive for Logstash version")
		}
	}

	if !fileExists(targetDirVersion) {
		s.log.Infof("Unarchive Logstash to %s", targetDirVersion)

		var u archiver.Unarchiver

		switch {
		case strings.HasSuffix(filename, "zip"):
			u = &archiver.Zip{
				MkdirAll:          true,
				OverwriteExisting: true,
				StripComponents:   1,
			}
		case strings.HasSuffix(filename, "tar.gz"):
			u = &archiver.TarGz{
				Tar: &archiver.Tar{
					MkdirAll:          true,
					OverwriteExisting: true,
					StripComponents:   1,
				},
			}
		default:
			return errors.New("file suffix not supported for unarchiving")
		}

		err = u.Unarchive(targetFile, targetDirVersion)
		if err != nil {
			return errors.Wrap(err, "failed to unarchive downloaded Logstash archive")
		}
	}

	s.log.Infof("Done, files available in %s", targetDirVersion)

	return nil
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func download(filename string, targetFile string) error {
	res, err := http.Get(fmt.Sprintf("https://artifacts.elastic.co/downloads/logstash/%s", filename))
	if err != nil {
		return errors.Wrap(err, "failed to download Logstash release")
	}
	defer res.Body.Close()

	file, err := os.OpenFile(targetFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to open target file for downloaded Logstash archive")
	}
	defer file.Close()

	bar := pb.Full.Start64(res.ContentLength)
	barReader := bar.NewProxyReader(res.Body)
	defer bar.Finish()

	_, err = io.Copy(file, barReader)
	if err != nil {
		return errors.Wrap(err, "failed to write target file for downloaded Logstash archive")
	}
	bar.Finish()

	return nil
}
