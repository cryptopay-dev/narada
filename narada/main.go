package main

import (
	"os"

	"github.com/cryptopay-dev/narada"
	"github.com/cryptopay-dev/narada/narada/commands"

	"github.com/sirupsen/logrus"

	"github.com/urfave/cli/v2"
)

const (
	ConsoleToolName    = "narada"
	ConsoleToolVersion = "0.1"
)

func main() {
	// Creating logger system
	logger := logrus.New()

	// Creating instance of Narada
	n := narada.New(narada.Options{
		Name:    ConsoleToolName,
		Version: ConsoleToolVersion,
	})

	// Creating urfave
	app := cli.NewApp()
	app.Name = ConsoleToolName
	app.Version = ConsoleToolVersion
	app.Description = "Narada CLI toolchain"
	app.Authors = []*cli.Author{
		&cli.Author{Name: "Pavel Makarenko", Email: "<cryfall@gmail.com>"},
	}
	app.Commands = []*cli.Command{
		commands.MigrateUp(n),
		commands.MigrateDown(n),
		commands.CreateMigration(n),
	}

	if err := app.Run(os.Args); err != nil {
		logger.Fatalf("error starting: %v", err)
	}
}
