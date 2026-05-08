package main

import (
	"log"
	"os"

	"github.com/abishekkumar/claude-memory/cli/cmd"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "memory",
		Usage: "Claude Memory — operational control plane for your memory platform.",
		Commands: []*cli.Command{
			cmd.DoctorCmd(),
			cmd.StatusCmd(),
			cmd.StatsCmd(),
			cmd.SearchCmd(),
			cmd.ExportCmd(),
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
