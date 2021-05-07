package controller

import (
	"sync"

	"github.com/tidwall/gjson"
)

type events struct {
	events            []string
	receivedUniqueIDs map[string]struct{}
	expected          int
	mutex             *sync.Mutex
}

func newEvents() *events {
	return &events{
		events:            make([]string, 0, 100),
		receivedUniqueIDs: make(map[string]struct{}, 100),
		mutex:             &sync.Mutex{},
	}
}

func (e *events) append(event string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.events = append(e.events, event)
	id := gjson.Get(event, `__lfv_id`).String()
	e.receivedUniqueIDs[id] = struct{}{}
}

func (e *events) isComplete() bool {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	return e.expected == len(e.receivedUniqueIDs)
}

func (e *events) reset(expected int) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.expected = expected
	e.events = make([]string, 0, 100)
	e.receivedUniqueIDs = make(map[string]struct{}, 100)
}

func (e *events) get() []string {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	results := make([]string, 0, len(e.events))
	results = append(results, e.events...)

	return results
}
