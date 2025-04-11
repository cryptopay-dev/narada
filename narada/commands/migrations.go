package commands

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/cryptopay-dev/narada"
	"github.com/cryptopay-dev/narada/clients"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"

	_ "github.com/lib/pq"
)

const DefaultMigrationsDir = "./migrations"

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

				ctx := context.Background()

				db := clients.NewPostgreSQL(v, logger)
				m, err := migrator(db, dir)
				if err != nil {
					return err
				}

				if err = m.Init(ctx); err != nil {
					return err
				}

				group, err := m.Migrate(ctx)
				if err != nil {
					return err
				}

				if group.ID == 0 {
					logger.Println("there are no new migrations to run")
					return nil
				}

				logger.Printf("migrated to %s\n", group)
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

				ctx := context.Background()

				db := clients.NewPostgreSQL(v, logger)
				m, err := migrator(db, dir)
				if err != nil {
					return err
				}

				if err = m.Init(ctx); err != nil {
					return err
				}

				group, err := m.Rollback(ctx)
				if err != nil {
					return err
				}

				if group.ID == 0 {
					logger.Println("there are no groups to rollback")
					return nil
				}

				logger.Printf("rolled back %s\n", group)
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
		},
		Action: func(c *cli.Context) error {
			p.Invoke(func(logger *logrus.Logger, v *viper.Viper) error {
				name := c.String("name")
				dir := c.String("dir")

				if name == "" {
					return errors.New("name cannot be empty")
				}
				name = strings.ReplaceAll(name, " ", "_")

				logger.Printf("creating sql migration: %s", name)

				db := clients.NewPostgreSQL(v, logger)
				m, err := migrator(db, dir)
				if err != nil {
					return err
				}

				files, err := m.CreateSQLMigrations(context.Background(), name)
				if err != nil {
					return err
				}

				for _, mf := range files {
					logger.Printf("created migration %s (%s)\n", mf.Name, mf.Path)
				}

				return nil
			})

			return nil
		},
	}
}

func migrator(db *bun.DB, dir string) (*migrate.Migrator, error) {
	migrations := migrate.NewMigrations(migrate.WithMigrationsDirectory(dir))
	if err := migrations.Discover(os.DirFS(dir)); err != nil {
		return nil, err
	}

	return migrate.NewMigrator(db, migrations), nil
}
