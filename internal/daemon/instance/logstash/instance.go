package logstash

import (
	"context"
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
	ctxKill     context.Context
	ctxShutdown context.Context

	controller *controller.Controller

	command string
	env     []string
	child   *exec.Cmd

	log logging.Logger

	logstashStarted    chan struct{}
	logstashShutdownWG *sync.WaitGroup
	shutdownWG         *sync.WaitGroup
}

func New(ctxKill context.Context, command string, env []string, log logging.Logger, shutdownWG *sync.WaitGroup) controller.Instance {
	return &instance{
		ctxKill:            ctxKill,
		command:            command,
		env:                env,
		log:                log,
		logstashStarted:    make(chan struct{}),
		logstashShutdownWG: &sync.WaitGroup{},
		shutdownWG:         shutdownWG,
	}
}

// start starts a Logstash child process with the previously supplied
// configuration.
func (i *instance) Start(ctx context.Context, controller *controller.Controller, workdir string) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	i.ctxShutdown = ctx
	defer func() {
		// if there has been an error during Start, cancel the context to signal
		// shutdown to all potentially running Go routines of instance.
		if err != nil {
			cancel()
		}
	}()

	i.controller = controller

	args := []string{
		"--path.settings",
		workdir,
		"--path.logs",
		workdir,
		"--path.data",
		workdir,
		// TODO: figure out the correct paths
		// "--path.plugins",
		// workdir,
		// "--path.config",
		// workdir,
	}

	i.child = exec.CommandContext(i.ctxKill, i.command, args...) // nolint: gosec
	i.child.Env = i.env
	stdout, err := i.child.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to setup stdoutPipe")
	}
	stderr, err := i.child.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "failed to setup stdoutPipe")
	}
	i.child.Stdin = &stdinBlockReader{
		ctx: i.ctxShutdown,
	}
	// Ensure a separate process group id for the Logstash child process, such
	// that signals like interrupt are not propagated automatically.
	i.child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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

func (i *instance) shutdownSignalHandler() {
	// Wait for shutdown signal coming from the daemon.
	<-i.ctxShutdown.Done()

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
