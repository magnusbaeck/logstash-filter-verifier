package controller

import "sync"

type pipelines struct {
	pipelines map[string]bool
	mutex     *sync.Mutex
}

func newPipelines() *pipelines {
	return &pipelines{
		pipelines: make(map[string]bool),
		mutex:     &sync.Mutex{},
	}
}

func (p *pipelines) reset(pipelines ...string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.pipelines = make(map[string]bool, len(pipelines)+1)
	p.pipelines["__lfv_pipelines_running"] = false

	for _, pipeline := range pipelines {
		p.pipelines[pipeline] = false
	}
}

func (p *pipelines) setReady(pipelines ...string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, pipeline := range pipelines {
		p.pipelines[pipeline] = true
	}
}

func (p *pipelines) isReady() bool {
	for _, ready := range p.pipelines {
		if !ready {
			return false
		}
	}
	return true
}
