package printing

import (
	"fmt"
	"io"
	"os"
)

const INFO int = 255
const WARNING int = 214
const ERROR int = 31
const SUCCESS int = 40
const GRAY int = 245

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

func PrintWithColoredForeground(writer io.Writer, color int, format string, args ...any) {
	if color < 0 || color > 255 {
		return
	}

	fmt.Fprintf(writer, "\033[2K\033[38;5;%dm", color)
	fmt.Fprintf(writer, format, args...)
	fmt.Fprintf(writer, "\033[0m\n")
}
