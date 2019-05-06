package narada

import (
	"context"
	"net/http"
	"testing"
	"time"

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
}
