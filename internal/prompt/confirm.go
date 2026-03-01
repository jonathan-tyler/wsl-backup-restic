package prompt

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type ConfirmFunc func(message string) (bool, error)

func NewYesNoConfirm(input io.Reader, output io.Writer) ConfirmFunc {
	reader := bufio.NewReader(input)

	return func(message string) (bool, error) {
		if _, err := fmt.Fprintf(output, "%s [y/N]: ", message); err != nil {
			return false, err
		}

		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return false, err
		}

		answer := strings.TrimSpace(strings.ToLower(line))
		return answer == "y" || answer == "yes", nil
	}
}
