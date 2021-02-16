package controller

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

type stateMachine struct {
	currentState stateName
	mutex        *sync.Mutex
	cond         *sync.Cond

	shutdown chan struct{}
	log      logging.Logger
}

func newStateMachine(shutdown chan struct{}, log logging.Logger) *stateMachine {
	mu := &sync.Mutex{}
	cond := sync.NewCond(mu)
	go func() {
		<-shutdown
		log.Debug("broadcast shutdown for waitForState")
		cond.Broadcast()
	}()
	return &stateMachine{
		currentState: stateCreated,
		mutex:        mu,
		cond:         cond,
		shutdown:     shutdown,
		log:          log,
	}
}

func (s *stateMachine) waitForState(target stateName) error {
	s.log.Debugf("waitForState: %v", target)
	// TODO: Add a timeout to exit if the expected state is not reached in due time.
	// Create go routine to wake up every 1 second,
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for s.currentState != target {
		s.cond.Wait()

		select {
		case <-s.shutdown:
			// TODO: Can we do this without error return?
			return errors.Errorf("shutdown while waiting for state: %s", target)
		default:
		}
	}
	return nil
}

func (s *stateMachine) executeCommand(command command) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	currentState := s.currentState
	tupple := stateCommand{currentState, command}
	newState := stateUnknown
	f := stateTransitionsTable[tupple]
	if f != nil {
		newState = f()
	}

	s.log.Debugf("state change: %q -> %q by %q", currentState, newState, command)
	s.currentState = newState
	s.cond.Broadcast()
}

type stateName string

const (
	stateUnknown stateName = "unknown"

	stateCreated       stateName = "created"
	stateStarted       stateName = "started"
	stateReady         stateName = "ready"
	stateSettingUpTest stateName = "setting_up_test"
	stateReadyForTest  stateName = "ready_for_test"
	stateExecutingTest stateName = "executing_test"
	stateRunningTest   stateName = "running_test"
)

func (s stateName) String() string {
	return string(s)
}

type command string

const (
	commandStart         command = "start"
	commandPipelineReady command = "pipeline-ready"
	commandSetupTest     command = "setup-test"
	commandExecuteTest   command = "execute-test"
	commandTestComplete  command = "test-complete"
	commandTeardown      command = "teardown"
)

type stateCommand struct {
	state stateName
	cmd   command
}

type transitionFunc func() stateName

var stateTransitionsTable = map[stateCommand]transitionFunc{
	// State Created
	{stateCreated, commandStart}: func() stateName { return stateStarted },

	// State Started
	{stateStarted, commandPipelineReady}: func() stateName { return stateReady },

	// State Ready
	{stateReady, commandPipelineReady}: func() stateName { return stateReady },
	{stateReady, commandSetupTest}:     func() stateName { return stateSettingUpTest },

	// State LoadingSUT
	{stateSettingUpTest, commandPipelineReady}: func() stateName { return stateReadyForTest },

	// State Ready for Test
	{stateReadyForTest, commandExecuteTest}: func() stateName { return stateExecutingTest },
	{stateReadyForTest, commandTeardown}:    func() stateName { return stateStarted },

	// State Starting Test
	{stateExecutingTest, commandPipelineReady}: func() stateName { return stateRunningTest },

	// State Running Test
	{stateRunningTest, commandPipelineReady}: func() stateName { return stateRunningTest },
	{stateRunningTest, commandTestComplete}:  func() stateName { return stateReadyForTest },
}
