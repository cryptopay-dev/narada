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

	logger := logrus.New()

	db, err := NewPostgreSQL(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}

	type StringResult struct {
		Message string
	}
	var res StringResult
	_, err = db.QueryOne(&res, "SELECT 'hello' AS message")
	if err != nil {
		t.Fatal(err)
	}

	if res.Message != "hello" {
		t.Error("unexpected message")
	}
}
