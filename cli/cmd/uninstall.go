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
				output.Warn("This will remove legacy user-scope MCP registration (claude mcp remove memory -s user), if present.")
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
				output.Warn("No user-scope memory MCP to remove (or claude CLI failed): " + string(out))
			} else {
				output.OK("Removed user-scope memory MCP registration (plugin MCP is unchanged).")
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
