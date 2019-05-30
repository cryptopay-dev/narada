package narada

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

// NewConfig creates a new configuration, by default config file will be `config.yml` in same directory you run application
// if you want to override it you should provide `NARADA_CONFIG` environment variable.
func NewConfig() (*viper.Viper, error) {
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetConfigType("yaml")

	// Finding a path of configuration
	path := os.Getenv("NARADA_CONFIG")
	if path == "" {
		v.SetConfigName("config")
		v.AddConfigPath(".")
	} else {
		v.SetConfigFile(path)
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	return v, nil
}
