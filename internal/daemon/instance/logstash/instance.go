package logstash

import (
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/hpcloud/tail"
	"github.com/pkg/errors"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/controller"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

type instance struct {
	controller *controller.Controller

	command string
	child   *exec.Cmd

	log logging.Logger

	logstashStarted    chan struct{}
	shutdown           chan struct{}
	instanceShutdown   chan struct{}
	logstashShutdownWG *sync.WaitGroup
	shutdownWG         *sync.WaitGroup
}

func New(command string, log logging.Logger, shutdown chan struct{}, shutdownWG *sync.WaitGroup) controller.Instance {
	return &instance{
		command:            command,
		log:                log,
		logstashStarted:    make(chan struct{}),
		shutdown:           shutdown,
		logstashShutdownWG: &sync.WaitGroup{},
		shutdownWG:         shutdownWG,
	}
}

// start starts a Logstash child process with the previously supplied
// configuration.
func (i *instance) Start(controller *controller.Controller, workdir string) error {
	i.controller = controller

	args := []string{
		"--path.settings",
		workdir,
	}

	i.child = exec.Command(i.command, args...)
	stdout, err := i.child.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to setup stdoutPipe")
	}
	stderr, err := i.child.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "failed to setup stdoutPipe")
	}
	i.instanceShutdown = make(chan struct{})
	i.child.Stdin = &stdinBlockReader{
		shutdown: i.instanceShutdown,
	}

	i.logstashShutdownWG.Add(2)
	go i.stdoutProcessor(stdout)
	go i.stderrProcessor(stderr)

	err = i.child.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start logstash process")
	}
	close(i.logstashStarted) // signal stdout and stderr scanners to start

	logfile := workdir + "/logstash.log"
	_ = os.Remove(logfile)
	t, err := tail.TailFile(logfile, tail.Config{Follow: true, Logger: tailLogger{i.log}})
	if err != nil {
		return errors.Wrap(err, "failed to read from logstash log file")
	}

	i.logstashShutdownWG.Add(1)
	go i.logstashLogProcessor(t)

	// stop process and free all allocated resources connected to this process.
	go i.shutdownSignalHandler()

	return nil
}

// TODO: What is needed for what?
func (i *instance) shutdownSignalHandler() {
	// Wait for shutdown signal coming from the daemon.
	<-i.shutdown

	i.Shutdown()
}

// TODO: What is needed for what?
func (i *instance) Shutdown() {
	close(i.instanceShutdown)

	i.stopLogstash()

	i.logstashShutdownWG.Wait()

	i.shutdownWG.Done()
}

func (i *instance) stopLogstash() {
	if i.child.Process == nil {
		return
	}

	err := i.child.Process.Signal(syscall.SIGTERM)
	if err != nil {
		i.log.Errorf("failed to send SIGTERM, Logstash might already be down:", err)
	}

	// TODO: Add timeout, then send syscall.SIGKILL
	err = i.child.Wait()
	if err != nil {
		i.log.Errorf("failed to wait for child process: %v", err)
	}
}

func (i *instance) ConfigReload() error {
	if i.child.Process == nil {
		return errors.New("can't signal to an unborn process")
	}

	err := i.child.Process.Signal(syscall.SIGHUP)
	if err != nil {
		return errors.Wrap(err, "failed to send SIGHUP")
	}

	return nil
}
