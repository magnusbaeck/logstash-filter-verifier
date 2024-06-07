package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/breml/logstash-config/ast"
	"github.com/breml/logstash-config/ast/astutil"
	"github.com/pkg/errors"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/idgen"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/logstashconfig"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pool"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/template"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/testcase"
)

type Session struct {
	id string

	logstashController         pool.LogstashController
	isOrderedPipelineSupported bool

	baseDir    string
	sessionDir string

	pipelines         pipeline.Pipelines
	inputPluginCodecs map[string]string
	testexec          int

	noCleanup bool

	log logging.Logger
}

func newSession(baseDir string, logstashController pool.LogstashController, noCleanup bool, isOrderedPipelineSupported bool, log logging.Logger) *Session {
	sessionID := idgen.New()
	sessionDir := fmt.Sprintf("%s/session/%s", baseDir, sessionID)
	return &Session{
		id:                         sessionID,
		baseDir:                    baseDir,
		sessionDir:                 sessionDir,
		logstashController:         logstashController,
		isOrderedPipelineSupported: isOrderedPipelineSupported,
		noCleanup:                  noCleanup,
		inputPluginCodecs:          map[string]string{},
		log:                        log,
	}
}

// ID returns the id of the session.
func (s Session) ID() string {
	return s.id
}

// setupTest prepares the Logstash configuration for a new test run.
func (s *Session) setupTest(pipelines pipeline.Pipelines, configFiles []logstashconfig.File) error {
	err := os.MkdirAll(s.sessionDir, 0700)
	if err != nil {
		return err
	}

	sutConfigDir := filepath.Join(s.sessionDir, "sut")

	// adjust pipeline names and config directories to session
	for i := range pipelines {
		pipelineName := fmt.Sprintf("lfv_%s_%s", s.id, pipelines[i].ID)

		pipelines[i].ID = pipelineName
		pipelines[i].Config = filepath.Join(sutConfigDir, pipelines[i].Config)
		if s.isOrderedPipelineSupported {
			pipelines[i].Ordered = "true"
		}
		pipelines[i].Workers = 1
	}

	// Preprocess and Save Config Files
	for _, configFile := range configFiles {
		inputCodecs, err := configFile.ReplaceInputs(s.id)
		if err != nil {
			return err
		}
		for id, codec := range inputCodecs {
			s.inputPluginCodecs[id] = codec
		}

		outputs, err := configFile.ReplaceOutputs()
		if err != nil {
			return err
		}

		err = configFile.Save(sutConfigDir)
		if err != nil {
			return err
		}

		outputPipelines, err := s.createOutputPipelines(outputs)
		if err != nil {
			return err
		}

		pipelines = append(pipelines, outputPipelines...)
	}

	// Reload Logstash Config
	s.pipelines = pipelines
	// err = s.logstash.ReloadPipelines(pipelines)
	err = s.logstashController.SetupTest(pipelines)
	if err != nil {
		s.log.Errorf("failed to reload Logstash config: %v", err)
	}

	return nil
}

func (s *Session) createOutputPipelines(outputs []string) ([]pipeline.Pipeline, error) {
	lfvOutputsDir := filepath.Join(s.sessionDir, "lfv_outputs")
	err := os.MkdirAll(lfvOutputsDir, 0700)
	if err != nil {
		return nil, err
	}

	pipelines := make([]pipeline.Pipeline, 0)
	for _, output := range outputs {
		pipelineName := fmt.Sprintf("lfv_output_%s", output)

		templateData := struct {
			PipelineName     string
			PipelineOrigName string
		}{
			PipelineName:     pipelineName,
			PipelineOrigName: output,
		}

		err = template.ToFile(filepath.Join(lfvOutputsDir, output+".conf"), outputPipeline, templateData, 0644)
		if err != nil {
			return nil, err
		}

		pipeline := pipeline.Pipeline{
			ID:      pipelineName,
			Config:  filepath.Join(lfvOutputsDir, output+".conf"),
			Workers: 1,
		}
		if s.isOrderedPipelineSupported {
			pipeline.Ordered = "true"
		}
		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

// ExecuteTest runs a test case set against the Logstash configuration, that has
// been loaded previously with SetupTest.
func (s *Session) ExecuteTest(inputPlugin string, inputLines []string, inEvents []map[string]interface{}, expectedEvents int) error {
	s.testexec++
	pipelineName := fmt.Sprintf("lfv_input_%d", s.testexec)
	inputDir := filepath.Join(s.sessionDir, "lfv_inputs", strconv.Itoa(s.testexec))
	inputPluginName := fmt.Sprintf("%s_%s_%s", "__lfv_input", s.id, inputPlugin)
	inputCodec, ok := s.inputPluginCodecs[inputPlugin]
	if !ok {
		inputCodec = "codec => plain"
	}

	// Prepare input directory
	err := os.MkdirAll(inputDir, 0700)
	if err != nil {
		return err
	}

	fieldsFilename := filepath.Join(inputDir, "fields.json")
	err = prepareFields(fieldsFilename, inEvents)
	if err != nil {
		return err
	}

	pipelineFilename := filepath.Join(inputDir, "input.conf")
	err = createInput(pipelineFilename, fieldsFilename, inputPluginName, inputLines, inputCodec)
	if err != nil {
		return err
	}

	pipeline := pipeline.Pipeline{
		ID:      pipelineName,
		Config:  pipelineFilename,
		Workers: 1,
	}
	if s.isOrderedPipelineSupported {
		pipeline.Ordered = "true"
	}
	pipelines := append(s.pipelines, pipeline)
	err = s.logstashController.ExecuteTest(pipelines, expectedEvents)
	if err != nil {
		return err
	}

	return nil
}

func prepareFields(fieldsFilename string, inEvents []map[string]interface{}) error {
	fields := make(map[string]map[string]interface{})

	for i, event := range inEvents {
		id := fmt.Sprintf("%d", i)
		fields[id] = event
	}

	bfields, err := json.Marshal(fields)
	if err != nil {
		return err
	}

	if err := os.WriteFile(fieldsFilename, bfields, 0600); err != nil {
		return err
	}

	return nil
}

func createInput(pipelineFilename string, fieldsFilename string, inputPluginName string, inputLines []string, inputCodec string) error {
	for i := range inputLines {
		var err error
		inputLine, err := astutil.Quote(inputLines[i], ast.DoubleQuoted)
		if err != nil {
			inputLine = astutil.QuoteWithEscape(inputLines[i], ast.SingleQuoted)
		}
		inputLines[i] = inputLine
	}

	templateData := struct {
		InputPluginName          string
		InputLines               string
		InputCodec               string
		FieldsFilename           string
		DummyEventInputIndicator string
	}{
		InputPluginName:          inputPluginName,
		InputLines:               strings.Join(inputLines, ", "),
		InputCodec:               inputCodec,
		FieldsFilename:           fieldsFilename,
		DummyEventInputIndicator: testcase.DummyEventInputIndicator,
	}
	err := template.ToFile(pipelineFilename, inputGenerator, templateData, 0600)
	if err != nil {
		return err
	}

	return nil
}

// GetResults returns the returned events from Logstash.
func (s *Session) GetResults() ([]string, error) {
	return s.logstashController.GetResults()
}

// GetStats returns the statistics for a test suite.
func (s *Session) GetStats() {
	panic("not implemented")
}

func (s *Session) teardown() error {
	// TODO: Perform a reset of the Logstash instance including Stdin Buffer, etc.
	err1 := s.logstashController.Teardown()
	var err2 error
	if !s.noCleanup {
		err2 = os.RemoveAll(s.sessionDir)
	}
	if err1 != nil || err2 != nil {
		return errors.Errorf("session teardown failed: %v, %v", err1, err2)
	}

	return nil
}
