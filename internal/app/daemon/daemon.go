package daemon

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"

	pb "github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/api/grpc"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/controller"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/instance/logstash"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/logstashconfig"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pool"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/session"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
	standalonelogstash "github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logstash"
)

type Daemon struct {
	// This context is passed to exec.Command. When killFunc is called,
	// the child process is killed with signal kill.
	killFunc context.CancelFunc

	// the ctxShutdownSignal allows shutdown request, that are received over
	// gRPC to signal shutdown to the shutdownSignalHandler.
	ctxShutdownSignal context.Context

	// shutdownSignalFunc is used by the shutdown gRPC handler to signal
	// shutdown to the shutdownSignalHandler.
	shutdownSignalFunc context.CancelFunc

	socket       string
	logstashPath string
	keptEnvVars  []string

	tempdir string

	inflightShutdownTimeout    time.Duration
	shutdownTimeout            time.Duration
	waitForStateTimeout        time.Duration
	waitForLateArrivalsTimeout time.Duration

	noCleanup bool

	sessionController *session.Controller

	server *grpc.Server

	log logging.Logger

	pb.UnimplementedControlServer
}

// New creates a new logstash filter verifier daemon.
func New(socket string, logstashPath string, keptEnvVars []string, log logging.Logger, inflightShutdownTimeout time.Duration, shutdownTimeout time.Duration, waitForStateTimeout time.Duration, noCleanup bool, waitForLateArrivalsTimeout time.Duration) Daemon {
	ctxShutdownSignal, shutdownSignalFunc := context.WithCancel(context.Background())
	return Daemon{
		socket:                     socket,
		logstashPath:               logstashPath,
		keptEnvVars:                keptEnvVars,
		inflightShutdownTimeout:    inflightShutdownTimeout,
		shutdownTimeout:            shutdownTimeout,
		log:                        log,
		ctxShutdownSignal:          ctxShutdownSignal,
		shutdownSignalFunc:         shutdownSignalFunc,
		waitForStateTimeout:        waitForStateTimeout,
		noCleanup:                  noCleanup,
		waitForLateArrivalsTimeout: waitForLateArrivalsTimeout,
	}
}

// Run starts the logstash filter verifier daemon.
func (d *Daemon) Run(ctx context.Context) error {
	// Two stage exit, cancel allows for graceful shutdown
	// kill exits sub processes with signal kill.
	ctxKill, killFunc := context.WithCancel(ctx)
	d.killFunc = killFunc
	ctx, shutdown := context.WithCancel(ctxKill)
	defer shutdown()

	tempdir, err := os.MkdirTemp("", "lfv-")
	if err != nil {
		return err
	}
	d.tempdir = tempdir
	d.log.Debugf("Temporary directory for daemon created in %q", d.tempdir)

	env := standalonelogstash.GetLimitedEnvironment(os.Environ(), d.keptEnvVars)

	logstashVersion, err := standalonelogstash.DetectVersion(d.logstashPath, env)
	if err != nil {
		return err
	}
	var isOrderedPipelineSupported bool
	if logstashVersion.Compare(semver.MustParse("v7.7.0")) >= 0 {
		isOrderedPipelineSupported = true
	}

	// Factory to create and start Logstash Controller
	shutdownLogstashInstancesWG := &sync.WaitGroup{}
	logstashControllerFactory := func() (session.LogstashController, error) {
		shutdownLogstashInstancesWG.Add(1)
		instance := logstash.New(ctxKill, d.logstashPath, env, d.log, shutdownLogstashInstancesWG)
		logstashController, err := controller.NewController(instance, tempdir, d.log, d.waitForStateTimeout, isOrderedPipelineSupported, d.waitForLateArrivalsTimeout)
		if err != nil {
			return nil, err
		}

		err = logstashController.Launch(ctx)
		if err != nil {
			return nil, err
		}

		return logstashController, nil
	}

	pool, err := pool.New(ctx, logstashControllerFactory, 2, d.log)
	if err != nil {
		return err
	}

	// Create Session Handler
	d.sessionController = session.NewController(d.tempdir, pool, d.noCleanup, isOrderedPipelineSupported, d.log)

	// Create and start GRPC Server
	lis, err := net.Listen("unix", d.socket)
	if err != nil {
		return err
	}
	d.server = grpc.NewServer()
	pb.RegisterControlServer(d.server, d)
	go func() {
		d.log.Infof("Daemon listening on %s", d.socket)
		err = d.server.Serve(lis)
		if err != nil {
			d.log.Errorf("failed to start daemon: %v", err)
			shutdown()
		}
	}()

	// Setup signal handler and shutdown coordinator
	d.shutdownSignalHandler(shutdown, shutdownLogstashInstancesWG)

	return nil
}

const hardExitDelay = 20 * time.Millisecond

func (d *Daemon) shutdownSignalHandler(shutdown func(), shutdownLogstashInstancesWG *sync.WaitGroup) {
	var hardExit bool

	defer func() {
		d.killFunc()
		if hardExit {
			// Give a little time to propagate Done to kill context
			time.Sleep(hardExitDelay)
			err := os.Remove(d.socket)
			if err != nil && !os.IsNotExist(err) {
				d.log.Warningf("failed to remove socket file %s during hard exit: %v", d.socket, err)
			}
		}
	}()

	// Listen to shutdown signal (coming from shutdown GRPC requests) as well
	// as OS signals interrupt and SIGTERM (not present on all systems).
	sigInt := make(chan os.Signal, 10)
	signal.Notify(sigInt, os.Interrupt)
	sigTerm := make(chan os.Signal, 10)
	signal.Notify(sigTerm, syscall.SIGTERM)

	select {
	case <-d.ctxShutdownSignal.Done():
		d.log.Info("Shutdown initiated.")
	case <-sigInt:
		d.log.Info("Interrupt signal (Ctrl+c) received. Shutdown initiated.")
		d.log.Info("Press Ctrl+c again to exit immediately")
	case <-sigTerm:
		d.log.Info("Term signal received. Shutdown initiated.")
	}

	t := time.NewTimer(d.inflightShutdownTimeout)

	// Wait for currently running sessions to finish.
	select {
	case <-d.sessionController.WaitFinish():
	case <-t.C:
		d.log.Debug("Wait for sessions timed out")
	case <-sigInt:
		d.log.Debug("Double interrupt signal received, exit now")
		hardExit = true
		return
	}
	// Stop timer and drain channel
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	shutdown()

	// Stop accepting new connections, wait for currently running handlers to finish properly.
	serverStopped := make(chan struct{})
	go func() {
		d.server.GracefulStop()
		close(serverStopped)
	}()

	// Stop Logstash instance
	logstashInstancesStopped := make(chan struct{})
	go func() {
		shutdownLogstashInstancesWG.Wait()
		close(logstashInstancesStopped)
	}()

	t.Reset(d.shutdownTimeout)

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
		case <-sigInt:
			d.log.Debug("Double interrupt signal received, exit now")
			hardExit = true
			return
		}
	}
	// Stop timer and drain channel
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}

// Cleanup removes the temporary files created by the daemon.
func (d *Daemon) Cleanup() {
	if d.noCleanup {
		return
	}

	err := os.RemoveAll(d.tempdir)
	if err != nil {
		d.log.Errorf("Failed to cleanup temporary directory for daemon %q: %v", d.tempdir, err)
	}
}

// Shutdown signals the daemon to shutdown.
func (d *Daemon) Shutdown(ctx context.Context, in *pb.ShutdownRequest) (*pb.ShutdownResponse, error) {
	select {
	case <-d.ctxShutdownSignal.Done():
		return nil, errors.New("daemon is already shutting down")
	default:
		d.shutdownSignalFunc()
	}

	return &pb.ShutdownResponse{}, nil
}

// SetupTest creates a new session, receives the pipeline configuration
// (zip archive), and prepares the files for the new session.
func (d *Daemon) SetupTest(ctx context.Context, in *pb.SetupTestRequest) (*pb.SetupTestResponse, error) {
	select {
	case <-d.ctxShutdownSignal.Done():
		return nil, errors.New("daemon is shutting down, no new sessions accepted")
	default:
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

			body, err := io.ReadAll(rc)
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
func (d *Daemon) ExecuteTest(ctx context.Context, in *pb.ExecuteTestRequest) (out *pb.ExecuteTestResponse, err error) {
	session, err := d.sessionController.Get(in.SessionID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid session ID")
	}

	defer func() {
		if err != nil {
			d.sessionController.DestroyByID(in.SessionID)
		}
	}()

	events := []map[string]interface{}{}
	err = json.Unmarshal(in.Events, &events)
	if err != nil {
		return nil, errors.Wrap(err, "invalid json for fields")
	}

	err = session.ExecuteTest(in.InputPlugin, in.InputLines, events, int(in.ExpectedEvents))
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
