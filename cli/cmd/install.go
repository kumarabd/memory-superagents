package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/urfave/cli/v2"
)

func InstallCmd() *cli.Command {
	return &cli.Command{
		Name:  "install",
		Usage: "Run install.sh to set up the full memory system",
		Action: func(c *cli.Context) error {
			exe, err := os.Executable()
			if err != nil {
				output.Fail("Cannot determine binary path: " + err.Error())
				return cli.Exit("", 1)
			}
			installSh := filepath.Join(filepath.Dir(exe), "..", "install.sh")
			if _, err := os.Stat(installSh); os.IsNotExist(err) {
				installSh = "./install.sh"
			}
			output.Info("Running " + installSh + "...")
			shell := "bash"
			if runtime.GOOS == "windows" {
				shell = "sh"
			}
			cmd := exec.Command(shell, installSh)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			if err := cmd.Run(); err != nil {
				output.Fail("install.sh failed: " + err.Error())
				return cli.Exit("", 1)
			}
			return nil
		},
	}
}
