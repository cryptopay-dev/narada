package tuktuk

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/spf13/viper"
	"go.uber.org/fx/fxtest"
)

func TestNewWorkers(t *testing.T) {
	logger := NewNopLogger()
	cfg := viper.New()
	lc := fxtest.NewLifecycle(t)

	w := NewWorkers(logger, cfg, lc)

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
}
