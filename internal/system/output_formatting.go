package system

import (
	"io"
	"strconv"
	"strings"
)

const (
	ansiDim   = "\x1b[2m"
	ansiReset = "\x1b[0m"
)

type ansiDimWriter struct {
	inner io.Writer
}

func (w ansiDimWriter) Write(value []byte) (int, error) {
	if len(value) == 0 {
		return 0, nil
	}
	if _, err := io.WriteString(w.inner, ansiDim); err != nil {
		return 0, err
	}
	written, err := w.inner.Write(value)
	if err != nil {
		return written, err
	}
	if _, err := io.WriteString(w.inner, ansiReset); err != nil {
		return written, err
	}
	return written, nil
}

func dimWriter(writer io.Writer) io.Writer {
	return ansiDimWriter{inner: writer}
}

func formatCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}

	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\n\"") {
			quoted = append(quoted, strconv.Quote(arg))
			continue
		}
		quoted = append(quoted, arg)
	}

	return strings.Join(quoted, " ")
}

func formatLine(name string, args []string) string {
	command := formatCommand(args)
	if command == "" {
		return name
	}
	return name + " " + command
}
