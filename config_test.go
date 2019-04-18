package tuktuk

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfig(t *testing.T) {
	t.Run("Loading configuration", func(t *testing.T) {
		t.Run("Failure on default config", func(t *testing.T) {
			cfg, err := NewConfig()
			assert.Nil(t, cfg)
			assert.Error(t, err)
		})

		t.Run("Success on configuration", func(t *testing.T) {
			os.Setenv("TUKTUK_CONFIG", "./fixtures/config.yml")
			defer os.Clearenv()

			cfg, err := NewConfig()
			assert.NotNil(t, cfg)
			assert.NoError(t, err)

			assert.Equal(t, ":8080", cfg.GetString("binding.api"))
		})
	})

	t.Run("Override configuration from env", func(t *testing.T) {
		os.Setenv("TUKTUK_CONFIG", "./fixtures/config.yml")
		os.Setenv("BINDING_API", ":8090")
		defer os.Clearenv()

		cfg, err := NewConfig()
		assert.NotNil(t, cfg)
		assert.NoError(t, err)

		assert.Equal(t, ":8090", cfg.GetString("binding.api"))
	})
}
