package commands

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/cryptopay-dev/narada/v2"
	"github.com/cryptopay-dev/narada/v2/clients"

	"github.com/pressly/goose/v3"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
)

const (
	DefaultMigrationsDir  = "./migrations"
	DefaultMigrationsType = "sql"

	migrationsDialect = "postgres"
)

// gooseLogger adapts a *slog.Logger to goose's printf-style Logger interface.
type gooseLogger struct {
	logger *slog.Logger
}

func (g gooseLogger) Printf(format string, v ...any) {
	g.logger.Info(strings.TrimRight(fmt.Sprintf(format, v...), "\n"))
}

func (g gooseLogger) Fatalf(format string, v ...any) {
	g.logger.Error(strings.TrimRight(fmt.Sprintf(format, v...), "\n"))
	os.Exit(1)
}

// setupGoose points goose at our logger and the postgres dialect.
func setupGoose(logger *slog.Logger) error {
	goose.SetLogger(gooseLogger{logger: logger})

	return goose.SetDialect(migrationsDialect)
}

// migrateUp opens a migration DB connection and applies all pending up migrations in dir.
func migrateUp(logger *slog.Logger, v *viper.Viper, dir string) error {
	db, err := clients.NewPostgreSQLForMigrations(v)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := setupGoose(logger); err != nil {
		return err
	}

	return goose.Up(db, dir)
}

// migrateDown rolls back the most recently applied migration in dir.
func migrateDown(logger *slog.Logger, v *viper.Viper, dir string) error {
	db, err := clients.NewPostgreSQLForMigrations(v)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := setupGoose(logger); err != nil {
		return err
	}

	return goose.Down(db, dir)
}

// migrateCreate scaffolds a new migration file of migrationType in dir.
func migrateCreate(logger *slog.Logger, dir, name, migrationType string) error {
	goose.SetLogger(gooseLogger{logger: logger})

	return goose.Create(nil, dir, name, migrationType)
}

func MigrateUp(p *narada.Narada) *cli.Command {
	return &cli.Command{
		Name: "migrate:up",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "dir", Value: DefaultMigrationsDir},
		},
		Action: func(c *cli.Context) error {
			p.Invoke(func(logger *slog.Logger, v *viper.Viper) error {
				logger.Info("starting migrations")
				if err := migrateUp(logger, v, c.String("dir")); err != nil {
					return err
				}
				logger.Info("finished migrating")
				return nil
			})

			return nil
		},
	}
}

func MigrateDown(p *narada.Narada) *cli.Command {
	return &cli.Command{
		Name: "migrate:down",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "dir", Value: DefaultMigrationsDir},
		},
		Action: func(c *cli.Context) error {
			p.Invoke(func(logger *slog.Logger, v *viper.Viper) error {
				logger.Info("rolling back migration")
				if err := migrateDown(logger, v, c.String("dir")); err != nil {
					return err
				}
				logger.Info("finished rollback")
				return nil
			})

			return nil
		},
	}
}

func CreateMigration(p *narada.Narada) *cli.Command {
	return &cli.Command{
		Name: "migrate:create",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name"},
			&cli.StringFlag{Name: "dir", Value: DefaultMigrationsDir},
			&cli.StringFlag{Name: "type", Value: DefaultMigrationsType},
		},
		Action: func(c *cli.Context) error {
			p.Invoke(func(logger *slog.Logger, v *viper.Viper) error {
				name := c.String("name")

				if name == "" {
					return errors.New("name cannot be empty")
				}

				logger.Info("creating sql migration", "name", name)
				return migrateCreate(logger, c.String("dir"), name, c.String("type"))
			})

			return nil
		},
	}
}
