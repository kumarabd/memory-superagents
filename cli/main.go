package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "memory",
		Usage: "Claude Memory — operational control plane for your memory platform.",
		Commands: []*cli.Command{},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
