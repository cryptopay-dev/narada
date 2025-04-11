package clients

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func TestNewPostgreSQL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	cfg := viper.New()
	cfg.Set("database.addr", os.Getenv("DATABASE_ADDR"))
	cfg.Set("database.user", os.Getenv("DATABASE_USER"))
	cfg.Set("database.password", os.Getenv("DATABASE_PASSWORD"))
	cfg.Set("database.database", os.Getenv("DATABASE_DATABASE"))
	cfg.Set("database.ssl", os.Getenv("DATABASE_SSL"))
	cfg.Set("database.path_to_ssl_root_cert", os.Getenv("PATH_TO_SSL_ROOT_CERT"))

	logger := logrus.New()

	db := NewPostgreSQL(cfg, logger)

	var res string
	err := db.QueryRow("SELECT 'hello'").Scan(&res)
	if err != nil {
		t.Error(err)
	}

	if res != "hello" {
		t.Fail()
	}
}
