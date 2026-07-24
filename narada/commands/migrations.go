package commands

import (
	"errors"

	"github.com/cryptopay-dev/narada"
	"github.com/cryptopay-dev/narada/clients"

	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"

	_ "github.com/lib/pq"
)

const (
	DefaultMigrationsDir  = "./migrations"
	DefaultMigrationsType = "sql"

	migrationsDialect = "postgres"
)

// migrateUp opens a migration DB connection and applies all pending up migrations in dir.
func migrateUp(logger *logrus.Logger, v *viper.Viper, dir string) error {
	db, err := clients.NewPostgreSQLForMigrations(v)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	goose.SetLogger(logger)
	if err := goose.SetDialect(migrationsDialect); err != nil {
		return err
	}

	return goose.Up(db, dir)
}

// migrateDown rolls back the most recently applied migration in dir.
func migrateDown(logger *logrus.Logger, v *viper.Viper, dir string) error {
	db, err := clients.NewPostgreSQLForMigrations(v)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	goose.SetLogger(logger)
	if err := goose.SetDialect(migrationsDialect); err != nil {
		return err
	}

	return goose.Down(db, dir)
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
				if err := migrateUp(logger, v, c.String("dir")); err != nil {
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
				if err := migrateDown(logger, v, c.String("dir")); err != nil {
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
