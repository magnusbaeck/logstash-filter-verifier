package daemon

import (
	"archive/zip"
	"bytes"
	"context"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"

	pb "github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/api/grpc"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/controller"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/instance/logstash"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/logstashconfig"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/session"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

type Daemon struct {
	socket       string
	logstashPath string

	tempdir string

	sessionController *session.Controller

	server             *grpc.Server
	logstashController *controller.Controller

	// This channel is closed as soon as the shutdown is in progress.
	shutdownInProgress chan struct{}

	// Global shutdownLogstashInstance channel, all Go routines should listen to this channel to
	// get notified and safely exit on shutdownLogstashInstance of the daemon.
	// The shutdownSignalHandler() will close this channel on shutdownLogstashInstance.
	shutdownLogstashInstance chan struct{}

	// shutdownSignal is sent by the Shutdown GRPC handler, when a shutdown command
	// is received. The shutdownSignal channel is processed by the shutdownSignalHandler().
	shutdownSignal chan struct{}

	// Global shutdown wait group. Daemon.Run() will wait for this wait group
	// before returning and exiting the main Go routine.
	shutdownLogstashInstancesWG *sync.WaitGroup

	log logging.Logger

	pb.UnimplementedControlServer
}

// New creates a new logstash filter verifier daemon.
func New(socket string, logstashPath string, log logging.Logger) Daemon {
	return Daemon{
		socket:                      socket,
		logstashPath:                logstashPath,
		log:                         log,
		shutdownInProgress:          make(chan struct{}),
		shutdownLogstashInstance:    make(chan struct{}),
		shutdownSignal:              make(chan struct{}),
		shutdownLogstashInstancesWG: &sync.WaitGroup{},
	}
}

// Run starts the logstash filter verifier daemon.
func (d *Daemon) Run() error {
	tempdir, err := ioutil.TempDir("", "lfv-")
	if err != nil {
		return err
	}
	d.tempdir = tempdir
	d.log.Debugf("Temporary directory for daemon created in %q", d.tempdir)

	// Create and start Logstash Controller
	d.shutdownLogstashInstancesWG.Add(1)
	instance := logstash.New(d.logstashPath, d.log, d.shutdownLogstashInstance, d.shutdownLogstashInstancesWG)
	logstashController, err := controller.NewController(instance, tempdir, d.log, d.shutdownLogstashInstance)
	if err != nil {
		return err
	}
	d.logstashController = logstashController

	err = d.logstashController.Launch()
	if err != nil {
		return err
	}

	// Create Session Handler
	d.sessionController = session.NewController(d.tempdir, d.logstashController, d.log)

	// Setup signal handler and shutdown coordinator
	shutdownHandlerCompleted := make(chan struct{})
	go d.shutdownSignalHandler(shutdownHandlerCompleted)

	// Create and start GRPC Server
	lis, err := net.Listen("unix", d.socket)
	if err != nil {
		return err
	}
	d.server = grpc.NewServer()
	pb.RegisterControlServer(d.server, d)

	d.log.Infof("Daemon listening on %s", d.socket)
	err = d.server.Serve(lis)

	// This is called from the main Go routine, so we have to wait for all others
	// to shutdown, before we can return and end the program/daemon.
	<-shutdownHandlerCompleted

	return err
}

func (d *Daemon) shutdownSignalHandler(shutdownHandlerCompleted chan struct{}) {
	// Make sure, shutdownHandlerCompleted channel is closed and main Go routine
	// exits cleanly.
	defer close(shutdownHandlerCompleted)

	// Listen to shutdown signal (comming from shutdown GRPC requests) as well
	// as OS signals interrupt and SIGTERM (not present on all systems).
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	select {
	case <-d.shutdownSignal:
	case <-c:
	}

	// Shutdown signal or OS signal received, start shutdown procedure
	// Signal shutdown to all
	close(d.shutdownInProgress)
	close(d.shutdownSignal)

	// TODO: Make shutdown timeout configurable
	t := time.NewTimer(3 * time.Second)

	// Wait for currently running sessions to finish.
	select {
	case <-d.sessionController.WaitFinish():
		t.Stop()
	case <-t.C:
	}

	// Stop accepting new connections, wait for currently running handlers to finish properly.
	serverStopped := make(chan struct{})
	go func() {
		d.server.GracefulStop()
		close(serverStopped)
	}()

	// Stop Logstash instance
	logstashInstancesStopped := make(chan struct{})
	go func() {
		close(d.shutdownLogstashInstance)
		d.shutdownLogstashInstancesWG.Wait()
		close(logstashInstancesStopped)
	}()

	// TODO: Make shutdown timeout configurable
	t.Reset(3 * time.Second)

	// Wait for Logstash and GRPC Server to shutdown
	serverStopComplete := false
	logstashInstanceStopComplete := false
	for !serverStopComplete || !logstashInstanceStopComplete {
		select {
		case <-t.C:
			d.log.Debug("Shutdown timeout reached, force shutdown.")
			d.server.Stop()
			serverStopComplete = true
			logstashInstanceStopComplete = true
		case <-serverStopped:
			d.log.Debug("server successfully stopped.")
			serverStopComplete = true
			serverStopped = nil
		case <-logstashInstancesStopped:
			d.log.Debug("logstash instance successfully stopped.")
			logstashInstanceStopComplete = true
			logstashInstancesStopped = nil
		}
	}
	t.Stop()
}

// Cleanup removes the temporary files created by the daemon.
func (d *Daemon) Cleanup() {
	err := os.RemoveAll(d.tempdir)
	if err != nil {
		d.log.Errorf("Failed to cleanup temporary directory for daemon %q: %v", d.tempdir, err)
	}
}

// Shutdown signals the daemon to shutdown.
func (d *Daemon) Shutdown(ctx context.Context, in *pb.ShutdownRequest) (*pb.ShutdownResponse, error) {
	select {
	case d.shutdownSignal <- struct{}{}:
	default:
	}

	return &pb.ShutdownResponse{}, nil
}

func (d *Daemon) isShutdownInProgress() bool {
	select {
	case <-d.shutdownInProgress:
		return true
	default:
	}
	return false
}

// SetupTest creates a new session, receives the pipeline configuration
// (zip archive), and prepares the files for the new session.
func (d *Daemon) SetupTest(ctx context.Context, in *pb.SetupTestRequest) (*pb.SetupTestResponse, error) {
	if d.isShutdownInProgress() {
		return nil, errors.New("daemon is shutting down, no new sessions accepted")
	}

	pipelines, configFiles, err := d.extractZip(in.Pipeline)
	if err != nil {
		return nil, err
	}

	session, err := d.sessionController.Create(pipelines, configFiles)
	if err != nil {
		return nil, err
	}

	return &pb.SetupTestResponse{
		SessionID: session.ID(),
	}, err
}

func (d *Daemon) extractZip(in []byte) (pipeline.Pipelines, []logstashconfig.File, error) {
	r, err := zip.NewReader(bytes.NewReader(in), int64(len(in)))
	if err != nil {
		return pipeline.Pipelines{}, nil, err
	}

	pipelines := pipeline.Pipelines{}
	configFiles := make([]logstashconfig.File, 0, len(r.File))
	for _, f := range r.File {
		err = func() (err error) {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer func() {
				errClose := rc.Close()
				if errClose != nil {
					err = errors.Wrapf(errClose, "failed to close file, underlying error: %v", err)
				}
			}()

			body, err := ioutil.ReadAll(rc)
			if err != nil {
				return err
			}

			switch f.Name {
			case "pipelines.yml":
				err = yaml.Unmarshal(body, &pipelines)
				if err != nil {
					return err
				}
			default:
				configFile := logstashconfig.File{
					Name: f.Name,
					Body: body,
				}
				configFiles = append(configFiles, configFile)
			}
			return nil
		}()
		if err != nil {
			return pipeline.Pipelines{}, nil, err
		}
	}

	return pipelines, configFiles, nil
}

// ExecuteTest runs a test case set against the Logstash configuration, that has
// been loaded previously with SetupTest.
func (d *Daemon) ExecuteTest(ctx context.Context, in *pb.ExecuteTestRequest) (*pb.ExecuteTestResponse, error) {
	session, err := d.sessionController.Get(in.SessionID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid session ID")
	}

	err = session.ExecuteTest(in.InputLines, in.Fields)
	if err != nil {
		return nil, err
	}

	results, err := session.GetResults()
	if err != nil {
		d.log.Errorf("failed to wait for Logstash results: %v", err)
	}

	return &pb.ExecuteTestResponse{
		Results: results,
	}, nil
}

// TeardownTest closes a test session, previously opened by SetupTest.
// After all test case sets are executed against the Logstash configuration,
// the test session needs to be closed.
func (d *Daemon) TeardownTest(ctx context.Context, in *pb.TeardownTestRequest) (*pb.TeardownTestResponse, error) {
	err := d.sessionController.DestroyByID(in.SessionID)
	if err != nil {
		return nil, errors.Wrap(err, "destroy of session failed")
	}

	result := pb.TeardownTestResponse{}
	return &result, err
}
