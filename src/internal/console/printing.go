package console

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

type Level int

const (
	INFO       Level = 255
	WARNING    Level = 214
	ERROR      Level = 196
	SUCCESS    Level = 40
	BACKGROUND Level = 245
)

type Config struct {
	Quiet bool
}

var defaultConfig Config = Config{
	Quiet: false,
}

var config Config

func Configure(conf Config) {
	config = conf
}

func Reset() {
	config = defaultConfig
}

func MoveCursorUpNLines(lines int) {
	if lines <= 0 {
		return
	}

	fmt.Fprintf(os.Stdout, "\033[%dA", lines)
}

func ClearCurrentLine() {
	fmt.Fprintf(os.Stdout, "\033[2K")
}

func ClearNLinesAndPositionCursorAtStart(lines int) {
	if lines <= 0 {
		return
	}

	MoveCursorUpNLines(lines)
	for range lines {
		ClearCurrentLine()
		fmt.Println() // Move down a line for it to be cleared or to return to start
	}
	MoveCursorUpNLines(lines)
}

func Printlnf(level Level, format string, args ...any) {
	if config.Quiet && level != ERROR {
		return
	}

	writer := os.Stdout
	if level == ERROR {
		writer = os.Stderr
	}

	fmt.Fprintf(writer, "\033[2K\033[38;5;%dm", level)
	fmt.Fprintf(writer, format, args...)
	fmt.Fprintf(writer, "\033[0m\n")
}

func PrintTable(titles []string, rows [][]string, padding int) {
	if config.Quiet {
		return
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', 0)
	fmt.Fprintln(writer, strings.Join(titles, "\t")+"\t")

	for _, row := range rows {
		fmt.Fprintln(writer, strings.Join(row, "\t")+"\t")
	}

	writer.Flush()
}
