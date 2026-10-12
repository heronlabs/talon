package terminal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Echo turns off the echo of what is typed and returns the function that turns it back on and whether it turned it off, which it does only on a terminal.
type Echo interface {
	Off() (restore func() error, hidden bool, err error)
}

// PromptService asks for values on the terminal, one question per line.
type PromptService struct {
	scanner *bufio.Scanner
	writer  io.Writer
	echo    Echo
}

// NewPromptService returns a PromptService that reads the answers from reader, writes the questions to writer and turns the echo off through echo while a secret is typed.
func NewPromptService(reader io.Reader, writer io.Writer, echo Echo) *PromptService {
	return &PromptService{scanner: bufio.NewScanner(reader), writer: writer, echo: echo}
}

// Ask asks for the value of name, showing its label, and returns the answer of the next line or, when it is empty, fallback; the answer of a secret is typed with the echo off.
func (s *PromptService) Ask(name, fallback string, secret bool) (string, error) {
	fmt.Fprintf(s.writer, "%s: ", label(name, fallback, secret))

	if secret {
		return s.readSecret(fallback)
	}

	return s.read(fallback)
}

// readSecret turns the echo off, reads the answer, ends its line when the echo was off, since the terminal did not echo the newline, and always turns the echo back on.
func (s *PromptService) readSecret(fallback string) (answer string, err error) {
	restore, hidden, err := s.echo.Off()
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, restore()) }()

	answer, err = s.read(fallback)
	s.endLine(hidden)

	return answer, err
}

// endLine writes a newline when hidden, so the next question starts on its own line.
func (s *PromptService) endLine(hidden bool) {
	if hidden {
		fmt.Fprintln(s.writer)
	}
}

// read returns the answer of the next line or, when it is empty, fallback.
func (s *PromptService) read(fallback string) (string, error) {
	s.scanner.Scan()
	if err := s.scanner.Err(); err != nil {
		return "", err
	}

	answer := strings.TrimSpace(s.scanner.Text())
	if answer == "" {
		return fallback, nil
	}

	return answer, nil
}

// label returns name followed by fallback in brackets, or **** for a secret, and only name when there is no fallback.
func label(name, fallback string, secret bool) string {
	if fallback == "" {
		return name
	}

	shown := fallback
	if secret {
		shown = "****"
	}

	return fmt.Sprintf("%s [%s]", name, shown)
}
