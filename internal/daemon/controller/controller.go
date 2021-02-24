package controller

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"sync"

	"gopkg.in/yaml.v2"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/idgen"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/template"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

const LogstashInstanceDirectoryPrefix = "logstash-instance"

type Controller struct {
	id string

	workDir string
	log     logging.Logger

	instance      Instance
	instanceReady *sync.Once
	shutdown      context.CancelFunc

	stateMachine   *stateMachine
	receivedEvents *events
	pipelines      *pipelines
}

func NewController(instance Instance, baseDir string, log logging.Logger) (*Controller, error) {
	id := idgen.New()

	workDir := path.Join(baseDir, LogstashInstanceDirectoryPrefix, id)

	templateData := struct {
		WorkDir string
	}{
		WorkDir: workDir,
	}

	err := os.MkdirAll(workDir, 0700)
	if err != nil {
		return nil, err
	}

	templates := map[string]string{
		"logstash.yml":      logstashConfig,
		"log4j2.properties": log4j2Config,
		"stdin.conf":        stdinPipeline,
		"output.conf":       outputPipeline,
	}

	for filename, tmpl := range templates {
		err = template.ToFile(path.Join(workDir, filename), tmpl, templateData, 0600)
		if err != nil {
			return nil, err
		}
	}

	controller := Controller{
		id:            id,
		workDir:       workDir,
		log:           log,
		instance:      instance,
		instanceReady: &sync.Once{},

		receivedEvents: newEvents(),
		pipelines:      newPipelines(),
	}

	err = controller.writePipelines()
	if err != nil {
		return nil, err
	}

	return &controller, nil
}

func (c *Controller) ID() string {
	return c.id
}

func (c *Controller) Launch(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.shutdown = cancel

	c.pipelines.reset("stdin", "output")
	c.stateMachine = newStateMachine(ctx, c.log)
	c.stateMachine.executeCommand(commandStart)

	err := c.instance.Start(ctx, c, c.workDir)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) SetupTest(pipelines pipeline.Pipelines) error {
	err := c.stateMachine.waitForState(stateReady)
	if err != nil {
		return err
	}

	c.stateMachine.executeCommand(commandSetupTest)

	return c.reload(pipelines, 0)
}

func (c *Controller) ExecuteTest(pipelines pipeline.Pipelines, expectedEvents int) error {
	err := c.stateMachine.waitForState(stateReadyForTest)
	if err != nil {
		return err
	}

	c.stateMachine.executeCommand(commandExecuteTest)

	return c.reload(pipelines, expectedEvents)
}

func (c *Controller) GetResults() ([]string, error) {
	err := c.stateMachine.waitForState(stateReadyForTest)
	if err != nil {
		return nil, err
	}

	return c.receivedEvents.get(), nil
}

func (c *Controller) Teardown() error {
	err := c.stateMachine.waitForState(stateReadyForTest)
	if err != nil {
		return err
	}

	c.stateMachine.executeCommand(commandTeardown)

	return c.reload(nil, 0)
}

func (c *Controller) reload(pipelines pipeline.Pipelines, expectedEvents int) error {
	err := c.writePipelines(pipelines...)
	if err != nil {
		return err
	}

	pipelineNames := make([]string, 0, len(pipelines))
	for _, pipeline := range pipelines {
		pipelineNames = append(pipelineNames, pipeline.ID)
	}

	c.receivedEvents.reset(expectedEvents)
	c.pipelines.reset(pipelineNames...)

	err = c.instance.ConfigReload()
	return err
}

func (c *Controller) writePipelines(pipelines ...pipeline.Pipeline) error {
	basePipelines := basePipelines(c.workDir)

	pipelines = append(basePipelines, pipelines...)

	pipelinesBody, err := yaml.Marshal(pipelines)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path.Join(c.workDir, "pipelines.yml"), pipelinesBody, 0600)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) ReceiveEvent(event string) error {
	c.receivedEvents.append(event)

	if c.receivedEvents.isComplete() {
		err := c.stateMachine.waitForState(stateRunningTest)
		if err != nil {
			return err
		}

		c.stateMachine.executeCommand(commandTestComplete)
	}

	return nil
}

func (c *Controller) PipelinesReady(pipelines ...string) {
	c.pipelines.setReady(pipelines...)
	if c.pipelines.isReady() {
		c.instanceReady.Do(func() {
			c.log.Info("Ready to process tests")
		})

		c.stateMachine.executeCommand(commandPipelineReady)
	}
}

func (c *Controller) SignalCrash() {
	c.stateMachine.executeCommand(commandCrash)
	c.Kill()
}

func (c *Controller) Kill() {
	c.shutdown()
}

func (c *Controller) IsHealthy() bool {
	return c.stateMachine.getState() != stateUnknown
}
