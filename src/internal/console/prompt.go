package console

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strings"
)

func Prompt(text string, expected []string) string {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(text)

		input, _ := reader.ReadString('\n')

		input = strings.Replace(input, "\n", "", -1)
		input = strings.Replace(input, "\r", "", -1)

		if slices.Contains(expected, input) || len(expected) <= 0 {
			return input
		}

		MoveCursorUpNLines(1)
		ClearCurrentLine()
	}
}
