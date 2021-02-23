package pool

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"
)

type LogstashController interface {
	SetupTest(pipelines pipeline.Pipelines) error
	ExecuteTest(pipelines pipeline.Pipelines, expectedEvents int) error
	GetResults() ([]string, error)
	Teardown() error
}

type LogstashControllerFactory func() (LogstashController, error)

type Pool struct {
	logstashControllerFactory LogstashControllerFactory
	maxControllers            int

	mutex                *sync.Mutex
	availableControllers []LogstashController
	assignedControllers  []LogstashController
}

func New(logstashControllerFactory LogstashControllerFactory, maxControllers int) (*Pool, error) {
	if maxControllers < 1 {
		maxControllers = 1
	}

	instance, err := logstashControllerFactory()
	if err != nil {
		return nil, err
	}

	return &Pool{
		logstashControllerFactory: logstashControllerFactory,
		maxControllers:            maxControllers,

		mutex:                &sync.Mutex{},
		availableControllers: []LogstashController{instance},
		assignedControllers:  []LogstashController{},
	}, nil
}

func (p *Pool) Get() (LogstashController, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

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

	return nil, errors.Errorf("No instance available from pool")
}

func (p *Pool) Return(instance LogstashController, clean bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for i := range p.assignedControllers {
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
	panic("instance not found in assigned controllers")
}
