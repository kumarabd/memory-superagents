package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/urfave/cli/v2"
)

func RestoreCmd() *cli.Command {
	return &cli.Command{
		Name:      "restore",
		Usage:     "Restore the memory database from a SQL backup",
		ArgsUsage: "<backup-file>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "Skip confirmation"},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return cli.Exit("Usage: memory restore <backup-file>", 1)
			}
			file := c.Args().First()
			if _, err := os.Stat(file); os.IsNotExist(err) {
				output.Fail("File not found: " + file)
				return cli.Exit("", 1)
			}
			if !c.Bool("yes") {
				output.Warn("This will DROP and recreate the claude_memory database.")
				fmt.Print("Continue? [y/N]: ")
				var ans string
				fmt.Scanln(&ans)
				if ans != "y" && ans != "Y" {
					fmt.Println("Aborted.")
					return nil
				}
			}
			fmt.Printf("Restoring from %s...\n", file)
			for _, sql := range []string{
				"DROP DATABASE IF EXISTS claude_memory;",
				"CREATE DATABASE claude_memory;",
			} {
				cmd := exec.Command("docker", "exec", "claude-memory-db",
					"psql", "-U", "postgres", "-c", sql)
				if out, err := cmd.CombinedOutput(); err != nil {
					output.Fail(string(out))
					return cli.Exit("", 1)
				}
			}
			data, err := os.ReadFile(file)
			if err != nil {
				output.Fail("Read failed: " + err.Error())
				return cli.Exit("", 1)
			}
			cmd := exec.Command("docker", "exec", "-i", "claude-memory-db",
				"psql", "-U", "postgres", "-d", "claude_memory")
			cmd.Stdin = strings.NewReader(string(data))
			if out, err := cmd.CombinedOutput(); err != nil {
				output.Fail("Restore failed: " + string(out))
				return cli.Exit("", 1)
			}
			output.OK("Restored from " + file)
			return nil
		},
	}
}
