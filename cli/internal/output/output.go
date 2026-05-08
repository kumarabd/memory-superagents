package output

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

var (
	green  = color.New(color.FgGreen).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	bold   = color.New(color.Bold).SprintFunc()
)

func OK(msg string)   { fmt.Println(green("✓") + " " + msg) }
func Fail(msg string) { fmt.Fprintln(os.Stderr, red("✗")+" "+msg) }
func Warn(msg string) { fmt.Println(yellow("!") + " " + msg) }
func Info(msg string) { fmt.Println("·  " + msg) }
func Bold(msg string) { fmt.Println(bold(msg)) }

// Table creates a styled table writer that renders to stdout.
func Table(headers ...string) table.Writer {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleLight)
	t.Style().Options.SeparateRows = false

	row := make(table.Row, len(headers))
	for i, h := range headers {
		row[i] = text.Bold.Sprint(h)
	}
	t.AppendHeader(row)
	return t
}
