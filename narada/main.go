package main

import (
	"log/slog"
	"os"

	"github.com/cryptopay-dev/narada/v2"
	"github.com/cryptopay-dev/narada/v2/narada/commands"

	"github.com/urfave/cli/v2"
)

const (
	ConsoleToolName    = "narada"
	ConsoleToolVersion = "0.1"
)

func main() {
	// Creating logger system
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

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
		{Name: "Pavel Makarenko", Email: "<cryfall@gmail.com>"},
	}
	app.Commands = []*cli.Command{
		commands.MigrateUp(n),
		commands.MigrateDown(n),
		commands.CreateMigration(n),
	}

	if err := app.Run(os.Args); err != nil {
		narada.Fatal(logger, "error starting", narada.Err(err))
	}
}
