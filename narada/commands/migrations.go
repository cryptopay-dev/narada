package commands

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/cryptopay-dev/narada"
	"github.com/cryptopay-dev/narada/clients"

	"github.com/pressly/goose"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"

	_ "github.com/lib/pq"
)

const (
	DefaultMigrationsDir  = "./migrations"
	DefaultMigrationsType = "sql"

	DefaultMigrationsLockTimeout = 15 * time.Minute

	// migrationsLockID is goose v3's default lock ID, so narada v1 and v2 migrators exclude each other.
	migrationsLockID         int64 = 4097083626
	migrationsLockTimeoutKey       = "database.migrations_lock_timeout"
	migrationsLockPeriod           = time.Second
)

var errMigrationsLockTimeout = errors.New("timed out waiting for migrations lock")

// withMigrationsLock runs fn while holding a Postgres session advisory lock, so concurrent `migrate:up` runs (e.g. one per ECS task) apply each migration exactly once.
func withMigrationsLock(ctx context.Context, logger *logrus.Logger, v *viper.Viper, db *sql.DB, fn func() error) error {
	v.SetDefault(migrationsLockTimeoutKey, DefaultMigrationsLockTimeout)
	deadline := time.Now().Add(v.GetDuration(migrationsLockTimeoutKey))

	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	for attempt := 0; ; attempt++ {
		var locked bool
		if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationsLockID).Scan(&locked); err != nil {
			return fmt.Errorf("acquire migrations lock: %w", err)
		}
		if locked {
			break
		}
		if attempt == 0 {
			logger.Println("waiting for migrations lock")
		}
		if time.Now().After(deadline) {
			return errMigrationsLockTimeout
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(migrationsLockPeriod):
		}
	}

	defer func() {
		if _, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationsLockID); err != nil {
			logger.WithError(err).Error("release migrations lock")
		}
	}()

	return fn()
}

func migrateUp(ctx context.Context, logger *logrus.Logger, v *viper.Viper, dir string) error {
	db, err := clients.NewPostgreSQLForMigrations(v)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	goose.SetLogger(logger)
	return withMigrationsLock(ctx, logger, v, db, func() error { return goose.Up(db, dir) })
}

func migrateDown(ctx context.Context, logger *logrus.Logger, v *viper.Viper, dir string) error {
	db, err := clients.NewPostgreSQLForMigrations(v)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	goose.SetLogger(logger)
	return withMigrationsLock(ctx, logger, v, db, func() error { return goose.Down(db, dir) })
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
				dir := c.String("dir")
				t := c.String("type")

				if name == "" {
					return errors.New("name cannot be empty")
				}

				logger.Printf("creating sql migration: %s", name)
				goose.SetLogger(logger)
				return goose.Create(nil, dir, name, t)
			})

			return nil
		},
	}
}
