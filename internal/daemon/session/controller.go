package session

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/logstashconfig"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/pipeline"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

// A Controller manages the sessions.
type Controller struct {
	mutex *sync.Mutex
	wg    *sync.WaitGroup
	cond  *sync.Cond

	// TODO: define specific type for sessionID
	sessions map[string]*Session
	finished bool

	tempdir      string
	logstashPool Pool
	log          logging.Logger
}

// NewController creates a new session Controller.
func NewController(tempdir string, logstashPool Pool, log logging.Logger) *Controller {
	mu := &sync.Mutex{}

	return &Controller{
		mutex: mu,
		wg:    &sync.WaitGroup{},
		cond:  sync.NewCond(mu),

		sessions: make(map[string]*Session, 10),

		tempdir:      tempdir,
		logstashPool: logstashPool,
		log:          log,
	}
}

// Create creates a new Session.
func (s *Controller) Create(pipelines pipeline.Pipelines, configFiles []logstashconfig.File) (*Session, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for len(s.sessions) != 0 {
		s.cond.Wait()
		if s.finished {
			return nil, errors.New("shutdown in progress")
		}
	}

	logstashController, err := s.logstashPool.Get()
	if err != nil {
		return nil, err
	}

	session := new(s.tempdir, logstashController, s.log)
	s.sessions[session.ID()] = session

	s.wg.Add(1)

	err = session.setupTest(pipelines, configFiles)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// Get returns an existing session by id.
func (s *Controller) Get(id string) (*Session, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	session, ok := s.sessions[id]
	if !ok {
		return nil, errors.Errorf("no valid session found for id %q", id)
	}
	return session, nil
}

// Destroy deletes an existing session.
func (s *Controller) DestroyByID(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	session, ok := s.sessions[id]
	if !ok {
		return errors.Errorf("no valid session found for id %q", id)
	}
	defer func() {
		delete(s.sessions, id)
		s.cond.Signal()
		s.wg.Done()
	}()

	err := session.teardown()
	if err != nil {
		s.logstashPool.Return(session.logstashController, false)
		return err
	}
	s.logstashPool.Return(session.logstashController, true)

	return nil
}

// WaitFinish waits for all currently running sessions to finish.
func (s *Controller) WaitFinish() chan struct{} {
	s.mutex.Lock()
	s.finished = true
	s.mutex.Unlock()

	s.cond.Broadcast()

	c := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(c)
	}()

	return c
}
