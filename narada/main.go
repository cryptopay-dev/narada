package main

import (
	"os"

	"github.com/m1ome/narada"
	"github.com/m1ome/narada/narada/commands"

	"github.com/urfave/cli"
)

const (
	ConsoleToolName    = "narada"
	ConsoleToolVersion = "0.1"
)

func main() {
	// Create narada instance
	n := narada.New(ConsoleToolName, ConsoleToolVersion)

	// Creating urfave
	api := cli.NewApp()
	api.Name = ConsoleToolName
	api.Version = ConsoleToolVersion
	api.Description = "Narada CLI toolsets"
	api.Author = "Pavel Makarenko <cryfall@gmail.com>"

	api.Commands = []cli.Command{
		commands.Migrate(n),
		commands.CreateMigration(n),
	}

	api.Run(os.Args)
}
