package controller

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

type stateMachine struct {
	ctx context.Context

	currentState stateName
	mutex        *sync.Mutex
	cond         *sync.Cond

	// TODO: Maybe allow different timeouts for different states.
	waitForStateTimeout time.Duration

	log logging.Logger
}

func newStateMachine(ctx context.Context, log logging.Logger, waitForStateTimeout time.Duration) *stateMachine {
	mu := &sync.Mutex{}
	cond := sync.NewCond(mu)
	go func() {
		defer cond.Broadcast()
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Debug("broadcast shutdown for waitForState")
				return
			case <-t.C:
				// Wakeup all waitForState to allow them to timeout
				cond.Broadcast()
			}
		}
	}()
	return &stateMachine{
		ctx: ctx,

		currentState: stateCreated,
		mutex:        mu,
		cond:         cond,

		waitForStateTimeout: waitForStateTimeout,

		log: log,
	}
}

func (s *stateMachine) waitForState(target stateName) error {
	s.log.Debugf("waitForState: %v", target)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	t := time.NewTimer(s.waitForStateTimeout)
	defer func() {
		// Stop timer and drain channel
		if !t.Stop() {
			select {
			case <-t.C:
			default:
			}
		}
	}()

	for s.currentState != target {
		select {
		case <-s.ctx.Done():
			// TODO: Can we do this without error return?
			return errors.Errorf("shutdown while waiting for state: %s", target)
		case <-t.C:
			s.log.Debugf("state change: %q because of waitForState timeout", stateUnknown)
			s.currentState = stateUnknown
			s.cond.Broadcast()
			return errors.Errorf("timed out while waiting for state: %s", target)
		default:
		}
		if s.currentState == stateUnknown {
			return errors.Errorf("state unknown, failed to wait for state: %s", target)
		}

		s.cond.Wait()
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

func (s *stateMachine) getState() stateName {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.currentState
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
	commandCrash         command = "crash"
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

	// State Unknown is the fallback state for all undefined state transitions.
	// {*, *}:  func() stateName { return stateUnknown },
}
