package narada

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cryptopay-dev/narada/lock"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"go.uber.org/fx/fxtest"
)

//
// Mocks here
//
type mockedLocker struct{}

func (mockedLocker) Obtain(name string, expire time.Duration) lock.Mutex {
	return &mockedMutex{}
}

type mockedMutex struct {
}

func (mockedMutex) Lock() (bool, error) {
	return true, nil
}

func (mockedMutex) Unlock() error {
	return nil
}

var redisAddr = os.Getenv("REDIS_ADDR")

func TestNewWorkers(t *testing.T) {
	t.Run("Basic jobs scheduling", func(t *testing.T) {
		logger := NewNopLogger()
		cfg := viper.New()
		lc := fxtest.NewLifecycle(t)
		lock := &mockedLocker{}

		w, err := NewWorkers(WorkersOptions{
			Logger: logger,
			Config: cfg,
			LC:     lc,
			Locker: lock,
		})
		assert.NoError(t, err)

		//Adding jobs
		wg := sync.WaitGroup{}
		wg.Add(6)

		a := make(chan bool, 1)
		b := make(chan bool, 2)

		// Second job is config driven
		cfg.Set("jobs.second.enabled", true)
		cfg.Set("jobs.second.period", 100)

		// Third job not running at all
		cfg.Set("jobs.third.enabled", false)
		failure := make(chan bool, 1)

		w.Add(Job{
			Name: "first",
			Handler: func() {
				a <- true
			},
			Period:      time.Millisecond * 100,
			Immediately: true,
		}, Job{
			Name: "second",
			Handler: func() {
				b <- true
			},
		}, Job{
			Name: "third",
			Handler: func() {
				failure <- true
			},
		})

		// Starting lifecycle
		go func() {
			err := lc.Start(context.Background())
			assert.NoError(t, err)
		}()

		// Fetching messages
		go func() {
			aCount := 0
			bCount := 0

			for {
				select {
				case <-a:
					if aCount <= 2 {
						wg.Done()
					}

					aCount++
				case <-b:
					if bCount <= 2 {
						wg.Done()
					}

					bCount++
				}
			}
		}()

		done := make(chan bool, 1)
		go func() {
			wg.Wait()
			done <- true
		}()

		select {
		case <-failure:
			t.Fatalf("executed job that should be disabled through configuration")
		case <-done:
			err := lc.Stop(context.Background())
			assert.NoError(t, err)
		case <-time.After(time.Second * 10):
			t.Fatalf("timeout exceeded")
		}
	})

	t.Run("Exclusive checks", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping test in short mode")
		}

		aCounter := make(chan bool, 1)
		bCounter := make(chan bool, 1)
		cCounter := make(chan bool, 1)
		counters := []int{0, 0, 0}

		// Instantiating counters
		wait := sync.WaitGroup{}
		wait.Add(1)

		go func() {
			for {
				select {
				case <-aCounter:
					counters[0] = counters[0] + 1
				case <-bCounter:
					counters[1] = counters[1] + 1
				case <-cCounter:
					counters[2] = counters[2] + 1
				}

				//fmt.Printf("#%v\n", counters)

				if counters[0] == 5 || counters[1] == 5 || counters[2] == 5 {
					wait.Done()
					return
				}
			}
		}()

		// Creating two worker groups and running them
		cfg := viper.New()
		cfg.Set("jobs.first.enabled", true)
		logger := NewNopLogger()

		// Mocking locker behaviour
		lcs := make([]*fxtest.Lifecycle, 0)

		for i := 0; i < 3; i++ {
			func(number int) {
				lc := fxtest.NewLifecycle(t)

				w, err := NewWorkers(WorkersOptions{
					Logger: logger,
					Config: cfg,
					Locker: lock.NewRedis(redis.NewClient(&redis.Options{
						Addr: redisAddr,
					})),
					LC: lc,
				})
				require.NoError(t, err)
				w.Add(Job{
					Name: "exclusive_job",
					Handler: func() {
						switch number {
						case 0:
							aCounter <- true
						case 1:
							bCounter <- true
						case 2:
							cCounter <- true
						}
					},
					Period:      time.Millisecond * 50,
					Exclusive:   true,
					Immediately: true,
				})

				// Starting lifecycle
				go func(lc *fxtest.Lifecycle) {
					err := lc.Start(context.Background())
					assert.NoError(t, err)
				}(lc)

				// Adding lcs
				lcs = append(lcs, lc)
			}(i)
		}

		done := make(chan bool, 1)
		go func() {
			wait.Wait()
			done <- true
		}()

		select {
		case <-done:
			for _, lc := range lcs {
				err := lc.Stop(context.Background())
				assert.NoError(t, err)
			}
		case <-time.After(time.Second):
			for _, lc := range lcs {
				err := lc.Stop(context.Background())
				assert.NoError(t, err)
			}

			t.Fatalf("timeout exceeded")
		}

		// Filtering
		fCounters := make([]int, 0)
		for _, counter := range counters {
			if counter > 0 {
				fCounters = append(fCounters, counter)
			}
		}

		assert.Equal(t, 1, len(fCounters))
	})
}
