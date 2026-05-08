package cmd

import (
	"fmt"
	"os/exec"

	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/urfave/cli/v2"
)

func UninstallCmd() *cli.Command {
	return &cli.Command{
		Name:  "uninstall",
		Usage: "Remove MCP registration and optionally drop the database",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "keep-data", Usage: "Keep PostgreSQL data (only unregister MCP)"},
			&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "Skip confirmation"},
		},
		Action: func(c *cli.Context) error {
			if !c.Bool("yes") {
				output.Warn("This will unregister the MCP server from Claude Code.")
				if !c.Bool("keep-data") {
					output.Warn("The PostgreSQL container and all data will also be removed.")
				}
				fmt.Print("Continue? [y/N]: ")
				var ans string
				fmt.Scanln(&ans)
				if ans != "y" && ans != "Y" {
					fmt.Println("Aborted.")
					return nil
				}
			}
			if out, err := exec.Command("claude", "mcp", "remove", "memory", "-s", "user").CombinedOutput(); err != nil {
				output.Warn("Could not unregister MCP: " + string(out))
			} else {
				output.OK("MCP server unregistered from Claude Code.")
			}
			if !c.Bool("keep-data") {
				exec.Command("docker", "compose", "down", "-v").Run()
				output.OK("PostgreSQL container and volume removed.")
			}
			fmt.Println("\nTo reinstall: ./install.sh")
			return nil
		},
	}
}
