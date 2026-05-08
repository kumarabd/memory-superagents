package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/urfave/cli/v2"
)

func BackupCmd() *cli.Command {
	return &cli.Command{
		Name:  "backup",
		Usage: "Dump the memory database to a SQL file",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "Output file path"},
		},
		Action: func(c *cli.Context) error {
			outFile := c.String("output")
			if outFile == "" {
				outFile = fmt.Sprintf("memory-backup-%s.sql",
					time.Now().Format("2006-01-02-150405"))
			}
			fmt.Printf("Backing up to %s...\n", outFile)
			out, err := exec.Command("docker", "exec", "claude-memory-db",
				"pg_dump", "-U", "postgres", "-d", "claude_memory", "--no-owner").Output()
			if err != nil {
				output.Fail("pg_dump failed: " + err.Error())
				return cli.Exit("", 1)
			}
			if err := os.WriteFile(outFile, out, 0644); err != nil {
				output.Fail("Write failed: " + err.Error())
				return cli.Exit("", 1)
			}
			info, _ := os.Stat(outFile)
			output.OK(fmt.Sprintf("Backup written to %s (%d bytes)", outFile, info.Size()))
			return nil
		},
	}
}
