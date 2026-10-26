package commands

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"time"

	"github.com/cryptopay-dev/narada"
	"github.com/cryptopay-dev/narada/clients"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"

	_ "github.com/lib/pq"
)

const (
	DefaultMigrationsDir  = "./migrations"
	DefaultMigrationsType = "sql"

	DefaultMigrationsLockTimeout = 15 * time.Minute

	migrationsLockTimeoutKey = "database.migrations_lock_timeout"
	migrationsLockPeriod     = time.Second
)

// newProvider builds a goose provider that holds a Postgres advisory lock while migrating, so concurrent `migrate:up` runs (e.g. one per ECS task) apply each migration exactly once.
func newProvider(logger *logrus.Logger, v *viper.Viper, db *sql.DB, dir string, opts ...goose.ProviderOption) (*goose.Provider, error) {
	v.SetDefault(migrationsLockTimeoutKey, DefaultMigrationsLockTimeout)

	attempts := uint64(v.GetDuration(migrationsLockTimeoutKey) / migrationsLockPeriod)
	locker, err := lock.NewPostgresSessionLocker(
		lock.WithLockTimeout(uint64(migrationsLockPeriod/time.Second), max(attempts, 1)),
	)
	if err != nil {
		return nil, err
	}

	opts = append([]goose.ProviderOption{
		goose.WithSessionLocker(locker),
		goose.WithLogger(logger),
		goose.WithVerbose(true),
	}, opts...)

	return goose.NewProvider(goose.DialectPostgres, db, os.DirFS(dir), opts...)
}

// migrateUp opens a migration DB connection and applies all pending up migrations in dir.
func migrateUp(ctx context.Context, logger *logrus.Logger, v *viper.Viper, dir string, opts ...goose.ProviderOption) error {
	db, err := clients.NewPostgreSQLForMigrations(v)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	p, err := newProvider(logger, v, db, dir, opts...)
	if errors.Is(err, goose.ErrNoMigrations) {
		logger.Printf("no migrations found in %s", dir)
		return nil
	}
	if err != nil {
		return err
	}

	_, err = p.Up(ctx)
	return err
}

// migrateDown rolls back the most recently applied migration in dir.
func migrateDown(ctx context.Context, logger *logrus.Logger, v *viper.Viper, dir string, opts ...goose.ProviderOption) error {
	db, err := clients.NewPostgreSQLForMigrations(v)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	p, err := newProvider(logger, v, db, dir, opts...)
	if err != nil {
		return err
	}

	_, err = p.Down(ctx)
	return err
}

// migrateCreate scaffolds a new migration file of migrationType in dir.
func migrateCreate(logger *logrus.Logger, dir, name, migrationType string) error {
	goose.SetLogger(logger)
	return goose.Create(nil, dir, name, migrationType)
}

func MigrateUp(p *narada.Narada) *cli.Command {
	return &cli.Command{
		Name: "migrate:up",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "dir", Value: DefaultMigrationsDir},
		},
		Action: func(c *cli.Context) error {
			p.Invoke(func(logger *logrus.Logger, v *viper.Viper) error {
				logger.Println("starting migrations")
				if err := migrateUp(c.Context, logger, v, c.String("dir")); err != nil {
					return err
				}
				logger.Println("finished migrating")
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
			p.Invoke(func(logger *logrus.Logger, v *viper.Viper) error {
				logger.Println("rolling back migration")
				if err := migrateDown(c.Context, logger, v, c.String("dir")); err != nil {
					return err
				}
				logger.Println("finished rollback")
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
			p.Invoke(func(logger *logrus.Logger, v *viper.Viper) error {
				name := c.String("name")

				if name == "" {
					return errors.New("name cannot be empty")
				}

				logger.Printf("creating sql migration: %s", name)
				return migrateCreate(logger, c.String("dir"), name, c.String("type"))
			})

			return nil
		},
	}
}
