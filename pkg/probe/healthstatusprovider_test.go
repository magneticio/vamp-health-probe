package probe_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/magneticio/vamp-health-probe/pkg/probe"
	. "github.com/smartystreets/goconvey/convey"
)

func TestEmptyCollector(t *testing.T) {
	Convey("Given empty health status collector", t, func() {
		s := probe.NewHealthStatusProvider(nil)
		So(s, ShouldNotBeNil)

		Convey("When status collected", func() {
			s.Collect()

			Convey("Status should be OK", func() {
				err := s.Get()
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestCollectorWithSingleCheck(t *testing.T) {
	Convey("Given health status collector", t, func() {
		var wg, checkwg sync.WaitGroup
		var res error
		s := probe.NewHealthStatusProvider(map[string]probe.HealthStatusChecker{
			"first": func() error {
				defer wg.Done()
				checkwg.Wait()
				return res
			}})

		Convey("When Get had been called before Collect and there is no last error", func() {
			Convey("Status should be OK", func() {
				err := s.Get()
				So(err, ShouldBeNil)
			})
		})

		Convey("When check function returns no error", func() {
			res = nil
			wg.Add(1)
			s.Collect()
			wg.Wait()

			Convey("Status should be OK", func() {
				err := s.Get()
				So(err, ShouldBeNil)
			})
		})

		Convey("When check function returns error", func() {
			res = errors.New("Some error")
			wg.Add(1)
			s.Collect()
			wg.Wait()

			Convey("Status should return error", func() {
				err := s.Get()
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, res.Error())

				Convey("And second call to Get should return same error", func() {
					err2 := s.Get()
					So(err2, ShouldResemble, err)
				})

				Convey("And subsequent check call doesn't return error", func() {
					res = nil
					wg.Add(1)
					s.Collect()
					wg.Wait()

					Convey("Status should be OK", func() {
						err := s.Get()
						So(err, ShouldBeNil)
					})
				})

				Convey("And subsequent check call wasn't completed", func() {
					res = nil
					checkwg.Add(1)
					s.Collect()

					Convey("Status should return same error", func() {
						err2 := s.Get()
						So(err2, ShouldResemble, err)
					})
				})

				Convey("And subsequent check call returns another error", func() {
					res = errors.New("Second error")
					wg.Add(1)
					s.Collect()
					wg.Wait()

					Convey("Status should return last error", func() {
						err := s.Get()
						So(err.Error(), ShouldContainSubstring, res.Error())
					})
				})
			})
		})
	})
}

func TestCollectorWithMultipleChecks(t *testing.T) {
	Convey("Given health status collector with 2 checkers", t, func() {
		var wg sync.WaitGroup
		var res1, res2 error
		s := probe.NewHealthStatusProvider(map[string]probe.HealthStatusChecker{
			"first": func() error {
				defer wg.Done()
				return res1
			},
			"second": func() error {
				defer wg.Done()
				return res2
			},
		})

		Convey("When both checkers return no error", func() {
			res1 = nil
			res2 = nil
			wg.Add(2)
			s.Collect()
			wg.Wait()

			Convey("Status should be OK", func() {
				err := s.Get()
				So(err, ShouldBeNil)
			})
		})

		Convey("When one of checkers returns error", func() {
			res1 = nil
			res2 = errors.New("Some error")
			wg.Add(2)
			s.Collect()
			wg.Wait()

			Convey("Status should return error", func() {
				err := s.Get()
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, res2.Error())
			})
		})

		Convey("When both of checkers return error", func() {
			res1 = errors.New("First error")
			res2 = errors.New("Second error")
			wg.Add(2)
			s.Collect()
			wg.Wait()

			Convey("Status should contain one of them", func() {
				err := s.Get()
				So(err, ShouldNotBeNil)
			})
		})
	})
}

func TestCollectorWithRepeatedCollect(t *testing.T) {
	Convey("Given health status collector", t, func() {
		var checks sync.WaitGroup
		var cnt int64
		s := probe.NewHealthStatusProvider(map[string]probe.HealthStatusChecker{
			"first": func() error {
				defer checks.Done()
				n := atomic.AddInt64(&cnt, 1)
				switch n {
				case 1:
					return errors.New("First error")
				case 2:
					return errors.New("Second error")
				}
				return nil
			}})

		Convey("When Collect is called more times that Get", func() {
			checks.Add(2)
			s.Collect()
			s.Collect()
			checks.Wait()

			Convey("Get returns latest result", func() {
				err := s.Get()
				So(err.Error(), ShouldContainSubstring, "Second error")
			})
		})
	})
}

func TestStartStop(t *testing.T) {
	Convey("Given health status collector", t, func() {
		var wg sync.WaitGroup
		cnt := 0
		var res error
		s := probe.NewHealthStatusProvider(map[string]probe.HealthStatusChecker{
			"first": func() error {
				cnt++
				if cnt == 3 {
					wg.Done()
				}
				return res
			}})

		Convey("When Start is called", func() {
			wg.Add(1)
			s.Start(100 * time.Millisecond)
			Convey("It keeps collecting data until Stop is called", func() {
				wg.Wait()
				err := s.Stop(time.Second)
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestStopWithTimeout(t *testing.T) {
	Convey("Given health status collector", t, func() {
		var wg sync.WaitGroup
		cnt := 0
		s := probe.NewHealthStatusProvider(map[string]probe.HealthStatusChecker{
			"first": func() error {
				if cnt == 0 {
					wg.Done()
					cnt = 1
				}
				time.Sleep(time.Second)
				return nil
			}})

		Convey("When checker func takes too long to complete", func() {
			Convey("And Start is called", func() {
				wg.Add(1)
				s.Start(100 * time.Millisecond)

				Convey("Stop should timed out", func() {
					wg.Wait()
					err := s.Stop(100 * time.Millisecond)
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, "timeout")
				})
			})
		})
	})
}

func TestHealthStatusServe(t *testing.T) {
	Convey("Giving health status collector", t, func() {
		var wg sync.WaitGroup
		var res error
		s := probe.NewHealthStatusProvider(map[string]probe.HealthStatusChecker{
			"first": func() error {
				defer wg.Done()
				return res
			}})

		Convey("When status check returns no error", func() {
			res = nil
			wg.Add(1)
			s.Collect()
			wg.Wait()

			Convey("Handler should return HTTP status OK", func() {
				req := httptest.NewRequest("GET", "http://localhost", nil)
				w := httptest.NewRecorder()
				s.Handler(w, req)
				resp := w.Result()
				So(resp.StatusCode, ShouldEqual, http.StatusOK)
			})
		})

		Convey("When status check returns error", func() {
			res = errors.New("Some error")
			wg.Add(1)
			s.Collect()
			wg.Wait()

			Convey("Handler should return HTTP error", func() {
				req := httptest.NewRequest("GET", "http://localhost", nil)
				w := httptest.NewRecorder()
				s.Handler(w, req)
				resp := w.Result()
				So(resp.StatusCode, ShouldEqual, http.StatusInternalServerError)
			})
		})
	})
}
