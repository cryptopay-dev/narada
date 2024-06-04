package narada

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	os.Setenv("NARADA_CONFIG", "./fixtures/config.yml")
	os.Setenv("BIND_API", ":12345")
	os.Setenv("LOGGER_LEVEL", "error")
	defer os.Clearenv()

	app := New(Options{
		Name:    "testing",
		Version: "dev",
	})

	t.Run("It should start on and run servers", func(t *testing.T) {
		errChan := make(chan error, 1)
		doneChan := make(chan bool, 1)

		go app.Start(func(s *Multiserver) error {
			mux := http.NewServeMux()
			mux.HandleFunc("/ping", func(writer http.ResponseWriter, request *http.Request) {
				doneChan <- true
				writer.WriteHeader(200)
			})

			if err := s.Add("api", mux); err != nil {
				errChan <- err
				return nil
			}

			return nil
		})

		go func() {
			for {
				time.Sleep(time.Millisecond * 50)
				if _, err := http.Get("http://127.0.0.1:12345/ping"); err != nil {
					continue
				}
			}
		}()

		select {
		case err := <-errChan:
			t.Fatalf("responded with error: %v", err)
		case <-doneChan:
			app.Stop()
		case <-time.After(time.Second * 5):
			t.Fatalf("timeout exceeded")
		}

	})

	t.Run("It allows to invoke one-shot tasks", func(t *testing.T) {
		value := 1

		app.Invoke(func(l *logrus.Logger) {
			l.Info("Changing value to 42")
			value = 42
		})

		assert.Equal(t, 42, value)
	})
}
