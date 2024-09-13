package narada

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"go.uber.org/fx/fxtest"
)

func TestNewMultiServers(t *testing.T) {
	t.Run("Adding", func(t *testing.T) {
		cfg := viper.New()
		logger := NewNopLogger()
		lc := fxtest.NewLifecycle(t)

		ms, err := NewMultiServers(cfg, logger, lc)
		assert.NotNil(t, ms)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ms.servers))

		t.Run("Success", func(t *testing.T) {
			cfg.Set("bind.api", ":8080")

			err := ms.Add("api", http.DefaultServeMux)
			assert.NoError(t, err)

			assert.True(t, len(ms.servers) > 0)
		})

		t.Run("Failure", func(t *testing.T) {
			err := ms.Add("unknown", http.DefaultServeMux)
			assert.Error(t, err)
		})

		t.Run("Duplicate", func(t *testing.T) {
			cfg.Set("bind.api2", ":8080")

			{
				err := ms.Add("api2", http.DefaultServeMux)
				assert.NoError(t, err)
			}

			{
				err := ms.Add("api2", http.DefaultServeMux)
				assert.Error(t, err)
			}
		})
	})

	t.Run("Server start & Stop", func(t *testing.T) {
		cfg := viper.New()
		logger := NewNopLogger()
		lc := fxtest.NewLifecycle(t)

		ms, err := NewMultiServers(cfg, logger, lc)
		assert.NotNil(t, ms)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ms.servers))

		// Creating http server
		done := make(chan bool, 1)
		mux := http.NewServeMux()
		mux.HandleFunc("/ping", func(rw http.ResponseWriter, r *http.Request) {
			done <- true
		})

		cfg.Set("bind.test", ":12346")
		{
			err := ms.Add("test", mux)
			assert.NoError(t, err)
		}

		go func() {
			err := lc.Start(context.Background())
			assert.NoError(t, err)
		}()

		go func() {
			for {
				_, err := http.Get("http://localhost:12346/ping")
				if err != nil {
					continue
				}

				break
			}
		}()

		select {
		case <-done:
			// Shutting down servers
			err := lc.Stop(context.Background())
			assert.NoError(t, err)
		case <-time.After(time.Second * 10):
			t.Fatalf("failure, timeout for server starting")
		}
	})

	t.Run("WithHealthcheck", func(t *testing.T) {
		cfg := viper.New()
		logger := logrus.New()
		lc := fxtest.NewLifecycle(t)

		ms, err := NewMultiServers(cfg, logger, lc)
		assert.NoError(t, err)

		cfg.Set("bind.test", ":12346")
		cfg.Set("bind.metrics", ":9002")

		err = NewMetricsInvoke(ms)
		assert.NoError(t, err)

		err = ms.Add(
			"test",
			http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.WriteHeader(http.StatusTeapot)
			}),
			WithHealthcheck("/health"),
		)
		assert.NoError(t, err)

		err = lc.Start(context.Background())
		assert.NoError(t, err)

		{
			res, err := http.Get("http://localhost:12346/1/ping")
			assert.NoError(t, err)
			assert.Equal(t, http.StatusTeapot, res.StatusCode)
		}

		{
			res, err := http.Get("http://localhost:12346/health1")
			assert.NoError(t, err)
			assert.Equal(t, http.StatusTeapot, res.StatusCode)
		}

		{
			res, err := http.Get("http://localhost:12346/health")
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, res.StatusCode)
		}

		// Check metrics
		{
			res, err := http.Get("http://localhost:9002")
			assert.NoError(t, err)

			body, err := io.ReadAll(res.Body)
			assert.NoError(t, err)

			assert.Contains(t, string(body), `server_healthcheck_duration_seconds_count{code="200",method="get",server="test"} 1`)
			assert.NotContains(t, string(body), `server_healthcheck_duration_seconds_count{code="418",method="get",server="test"}`)
		}

		err = lc.Stop(context.Background())
		assert.NoError(t, err)
	})

	t.Run("AddHealthcheck", func(t *testing.T) {
		cfg := viper.New()
		logger := logrus.New()
		lc := fxtest.NewLifecycle(t)

		ms, err := NewMultiServers(cfg, logger, lc)
		assert.NoError(t, err)

		cfg.Set("bind.test_success", ":12346")
		cfg.Set("bind.test_failure", ":12347")
		cfg.Set("bind.metrics", ":9002")

		err = NewMetricsInvoke(ms)
		assert.NoError(t, err)

		err = ms.AddHealthcheck("test_success", "/healthz", func() error { return nil })
		assert.NoError(t, err)

		err = ms.AddHealthcheck("test_failure", "/healthz", func() error { return errors.New("test") })
		assert.NoError(t, err)

		err = lc.Start(context.Background())
		assert.NoError(t, err)

		{
			res, err := http.Get("http://localhost:12346")
			assert.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, res.StatusCode)
		}

		{
			res, err := http.Get("http://localhost:12346/health")
			assert.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, res.StatusCode)
		}

		{
			res, err := http.Get("http://localhost:12346/healthz")
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, res.StatusCode)
		}

		{
			res, err := http.Get("http://localhost:12347")
			assert.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, res.StatusCode)
		}

		{
			res, err := http.Get("http://localhost:12347/healthz")
			assert.NoError(t, err)
			assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		}

		// Check metrics
		{
			res, err := http.Get("http://localhost:9002")
			assert.NoError(t, err)

			body, err := io.ReadAll(res.Body)
			assert.NoError(t, err)

			assert.Contains(t, string(body), `server_healthcheck_duration_seconds_count{code="404",method="get",server="test_success"} 2`)
			assert.Contains(t, string(body), `server_healthcheck_duration_seconds_count{code="200",method="get",server="test_success"} 1`)
			assert.Contains(t, string(body), `server_healthcheck_duration_seconds_count{code="404",method="get",server="test_failure"} 1`)
			assert.Contains(t, string(body), `server_healthcheck_duration_seconds_count{code="500",method="get",server="test_failure"} 1`)
		}

		err = lc.Stop(context.Background())
		assert.NoError(t, err)
	})
}
