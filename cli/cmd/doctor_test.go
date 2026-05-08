package cmd_test

import (
	"testing"

	"github.com/abishekkumar/claude-memory/cli/cmd"
	"github.com/urfave/cli/v2"
)

func TestDoctorCmdHasCorrectName(t *testing.T) {
	c := cmd.DoctorCmd()
	if c.Name != "doctor" {
		t.Errorf("expected name 'doctor', got %q", c.Name)
	}
}

func TestDoctorCmdHasUsage(t *testing.T) {
	c := cmd.DoctorCmd()
	if c.Usage == "" {
		t.Error("doctor command must have a Usage string")
	}
}

func appWithDoctor() *cli.App {
	return &cli.App{
		Name:     "memory",
		Commands: []*cli.Command{cmd.DoctorCmd()},
	}
}

func TestDoctorHelpFlag(t *testing.T) {
	app := appWithDoctor()
	err := app.Run([]string{"memory", "doctor", "--help"})
	if err != nil {
		t.Errorf("doctor --help returned error: %v", err)
	}
}
