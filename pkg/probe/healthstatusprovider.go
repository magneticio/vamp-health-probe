package probe

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// HealthStatusChecker - single function used to check health status
type HealthStatusChecker func() error

// HealthStatusProvider - defines health status provider
type HealthStatusProvider interface {
	Handler(w http.ResponseWriter, r *http.Request)
	Collect()
	Start(time.Duration)
	Stop(tm time.Duration) error
	Get() error
}

// HealthStatusProviderOption - option for healthStatus
type HealthStatusProviderOption func(*healthStatus)

// Logger - defines a logger used in HealthStatusProvider
type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Error(args ...interface{})
}

// WithLogger - option to replace default stdLogger with a custom logger
func WithLogger(logger Logger) HealthStatusProviderOption {
	return func(provider *healthStatus) {
		provider.logger = logger
	}
}

type stdLogger struct{}

func (logger *stdLogger) Debug(args ...interface{}) {
	fmt.Println([]interface{}{"Debug: ", args}...)
}
func (logger *stdLogger) Info(args ...interface{}) {
	fmt.Println([]interface{}{"Info: ", args}...)
}
func (logger *stdLogger) Error(args ...interface{}) {
	fmt.Println([]interface{}{"Error: ", args}...)
}

type healthStatus struct {
	checkers map[string]HealthStatusChecker
	results  map[string]*checkresult
	stopper  chan struct{}
	checkwg  *sync.WaitGroup
	logger   Logger
}

type checkresult struct {
	last error
	cur  chan error
}

// NewHealthStatusProvider - returns new HealthStatusProvider
func NewHealthStatusProvider(checkers map[string]HealthStatusChecker, options ...HealthStatusProviderOption) HealthStatusProvider {
	s := &healthStatus{
		checkers: make(map[string]HealthStatusChecker, len(checkers)),
		results:  make(map[string]*checkresult, len(checkers)),
		stopper:  make(chan struct{}),
		checkwg:  &sync.WaitGroup{},
	}
	for _, option := range options {
		option(s)
	}
	if s.logger == nil {
		s.logger = &stdLogger{}
	}
	for k, v := range checkers {
		s.checkers[k] = v
		s.results[k] = &checkresult{
			cur: make(chan error),
		}
	}
	return s
}

func (s *healthStatus) Collect() {
	for n, c := range s.checkers {
		s.checkwg.Add(1)
		go func(f HealthStatusChecker, ch chan error) {
			defer s.checkwg.Done()
			ch <- f()
		}(c, s.results[n].cur)
	}
}

func (s *healthStatus) Get() error {
	for n, res := range s.results {
		var r error
		for empty := false; !empty; {
			select {
			case r = <-res.cur:
				res.last = r
			default:
				empty = true
				r = res.last
			}
		}
		if r != nil {
			err := fmt.Errorf("Health check of %v failed: %v", n, r)
			s.logger.Error(err)
			return err
		}
	}
	return nil
}

func (s *healthStatus) Start(d time.Duration) {
	ticker := time.NewTicker(d)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.logger.Debug("Collecting health statuses")
				s.Get()
				s.Collect()
			case <-s.stopper:
				return
			}
		}
	}()
}

// Stop stops collecting health check results
// and wait for all checkers completion
// It can be called only once after Start has been called
func (s *healthStatus) Stop(tm time.Duration) error {
	close(s.stopper)

	c := make(chan struct{})
	go func() {
		defer close(c)
		s.checkwg.Wait()
	}()
	after := time.After(tm)
	for {
		select {
		case <-c:
			s.logger.Info("Health status provider stopped")
			return nil
		case <-after:
			s.logger.Error("Health status provider stopping timeout")
			return fmt.Errorf("stop timeout (%v)", tm)
		default:
			s.Get()
		}
	}
}

func (s *healthStatus) Handler(w http.ResponseWriter, r *http.Request) {
	if err := s.Get(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
