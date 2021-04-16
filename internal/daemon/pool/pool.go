package pool

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

type LogstashController interface {
	SetupTest(pipelines pipeline.Pipelines) error
	ExecuteTest(pipelines pipeline.Pipelines, expectedEvents int) error
	GetResults() ([]string, error)
	Teardown() error
	IsHealthy() bool
	Kill()
}

type LogstashControllerFactory func() (LogstashController, error)

type Pool struct {
	logstashControllerFactory LogstashControllerFactory
	maxControllers            int

	mutex                *sync.Mutex
	availableControllers []LogstashController
	assignedControllers  []LogstashController

	log logging.Logger
}

func New(ctx context.Context, logstashControllerFactory LogstashControllerFactory, maxControllers int, log logging.Logger) (*Pool, error) {
	if maxControllers < 1 {
		maxControllers = 1
	}

	instance, err := logstashControllerFactory()
	if err != nil {
		return nil, err
	}

	p := &Pool{
		logstashControllerFactory: logstashControllerFactory,
		maxControllers:            maxControllers,

		mutex:                &sync.Mutex{},
		availableControllers: []LogstashController{instance},
		assignedControllers:  []LogstashController{},

		log: log,
	}

	go p.housekeeping(ctx)

	return p, nil
}

// housekeeping removes unhealthy instances, ensure at least one running instance.
func (p *Pool) housekeeping(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()

	for {
		select {
		case <-t.C:
		case <-ctx.Done():
			return
		}
		func() {
			p.mutex.Lock()
			defer p.mutex.Unlock()

			// Remove unhealthy instances
			for i := 0; i < len(p.availableControllers); i++ {
				if !p.availableControllers[i].IsHealthy() {
					// Delete without preserving order
					p.availableControllers[i].Kill()
					p.availableControllers[i] = p.availableControllers[len(p.availableControllers)-1]
					p.availableControllers = p.availableControllers[:len(p.availableControllers)-1]
					i--
				}
			}
			for i := 0; i < len(p.assignedControllers); i++ {
				if !p.assignedControllers[i].IsHealthy() {
					// Delete without preserving order
					p.assignedControllers[i].Kill()
					p.assignedControllers[i] = p.assignedControllers[len(p.assignedControllers)-1]
					p.assignedControllers = p.assignedControllers[:len(p.assignedControllers)-1]
					i--
				}
			}

			if len(p.availableControllers)+len(p.assignedControllers) == 0 {
				instance, err := p.logstashControllerFactory()
				if err != nil {
					p.log.Warning("logstash pool housekeeping failed to start new instance: %v", err)
				}
				p.availableControllers = append(p.availableControllers, instance)
			}
		}()
	}
}

func (p *Pool) Get() (LogstashController, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Remove unhealthy instances
	for i := 0; i < len(p.availableControllers); i++ {
		if !p.availableControllers[i].IsHealthy() {
			// Delete without preserving order
			p.availableControllers[i].Kill()
			p.availableControllers[i] = p.availableControllers[len(p.availableControllers)-1]
			p.availableControllers = p.availableControllers[:len(p.availableControllers)-1]
			i--
		}
	}

	if len(p.availableControllers) > 0 {
		instance := p.availableControllers[0]
		p.availableControllers = p.availableControllers[1:]
		p.assignedControllers = append(p.assignedControllers, instance)
		return instance, nil
	}

	if len(p.assignedControllers) < p.maxControllers {
		instance, err := p.logstashControllerFactory()
		if err != nil {
			return nil, err
		}
		p.assignedControllers = append(p.assignedControllers, instance)
		return instance, nil
	}

	return nil, errors.Errorf("no instance available from pool")
}

func (p *Pool) Return(instance LogstashController, clean bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for i := 0; i < len(p.assignedControllers); i++ {
		if p.assignedControllers[i] == instance {
			// Delete without preserving order
			p.assignedControllers[i] = p.assignedControllers[len(p.assignedControllers)-1]
			p.assignedControllers = p.assignedControllers[:len(p.assignedControllers)-1]

			if clean {
				p.availableControllers = append(p.availableControllers, instance)
			}

			return
		}
	}
	p.log.Warning("Instance not found in assigned controllers. Instance might have been cleaned up by housekeeping due to unhealthy state.")
}
