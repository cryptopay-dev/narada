package narada

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

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
