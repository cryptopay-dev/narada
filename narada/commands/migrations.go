package commands

import (
	"errors"

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
)

func MigrateUp(p *narada.Narada) *cli.Command {
	return &cli.Command{
		Name: "migrate:up",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "dir", Value: DefaultMigrationsDir},
		},
		Action: func(c *cli.Context) error {
			p.Invoke(func(logger *logrus.Logger, v *viper.Viper) error {
				logger.Println("starting migrations")
				dir := c.String("dir")

				db, err := clients.NewPostgreSQLForMigrations(v)
				if err != nil {
					return err
				}

				goose.SetLogger(logger)
				if err := goose.Up(db, dir); err != nil {
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
				dir := c.String("dir")

				db, err := clients.NewPostgreSQLForMigrations(v)
				if err != nil {
					return err
				}

				goose.SetLogger(logger)
				if err := goose.Down(db, dir); err != nil {
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
