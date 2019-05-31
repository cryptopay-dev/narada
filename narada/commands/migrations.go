package commands

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/m1ome/narada"

	"github.com/pressly/goose"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/urfave/cli"

	_ "github.com/lib/pq"
)

const (
	DefaultMigrationsDir  = "./migrations"
	DefaultMigrationsType = "sql"
)

func Migrate(p *narada.Narada) cli.Command {
	return cli.Command{
		Name: "migrate:up",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "dir"},
		},
		Action: func(c *cli.Context) error {
			return Run(context.Background(), func(logger *logrus.Logger, v *viper.Viper) error {
				logger.Println("starting migrations")
				dir := c.String("dir")

				if dir == "" {
					dir = DefaultMigrationsDir
				}

				conn := fmt.Sprintf(
					"postgres://%s:%s@%s/%s?sslmode=disable",
					v.GetString("database.user"),
					v.GetString("database.password"),
					v.GetString("database.addr"),
					v.GetString("database.database"),
				)

				db, err := sql.Open("postgres", conn)
				if err != nil {
					return err
				}

				goose.SetLogger(logger)
				if err := goose.Run("up", db, dir); err != nil {
					return err
				}

				logger.Println("finished migrating")
				return nil
			})

			return nil
		},
	}
}

func CreateMigration(p *narada.Narada) cli.Command {
	return cli.Command{
		Name: "migrate:create",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "dir"},
			cli.StringFlag{Name: "name"},
			cli.StringFlag{Name: "type"},
		},
		Action: func(c *cli.Context) error {
			return Run(context.Background(), func(logger *logrus.Logger, v *viper.Viper) error {
				name := c.String("name")
				dir := c.String("dir")
				t := c.String("type")

				if name == "" {
					return errors.New("name cannot be empty")
				}

				if dir == "" {
					dir = DefaultMigrationsDir
				}

				if t == "" {
					t = DefaultMigrationsType
				}

				logger.Printf("creating sql migration: %s", name)
				goose.SetLogger(logger)
				return goose.Run("create", nil, dir, name, t)
			})

			return nil
		},
	}
}
