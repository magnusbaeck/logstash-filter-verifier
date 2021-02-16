package controller

import "sync"

type events struct {
	events   []string
	expected int
	mutex    *sync.Mutex
}

func newEvents() *events {
	return &events{
		events: make([]string, 0, 100),
		mutex:  &sync.Mutex{},
	}
}

func (e *events) append(event string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.events = append(e.events, event)
}

func (e *events) isComplete() bool {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	return e.expected == len(e.events)
}

func (e *events) reset(expected int) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.expected = expected
	e.events = make([]string, 0, 100)
}

func (e *events) get() []string {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	results := make([]string, 0, len(e.events))
	results = append(results, e.events...)

	return results
}
