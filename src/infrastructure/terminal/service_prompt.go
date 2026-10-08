package terminal

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// PromptService asks for values on the terminal, one question per line.
type PromptService struct {
	scanner *bufio.Scanner
	writer  io.Writer
}

// NewPromptService returns a PromptService that reads the answers from reader and writes the questions to writer.
func NewPromptService(reader io.Reader, writer io.Writer) *PromptService {
	return &PromptService{scanner: bufio.NewScanner(reader), writer: writer}
}

// Ask asks for the value of name, showing its label, and returns the answer of the next line or, when it is empty, fallback.
func (s *PromptService) Ask(name, fallback string, secret bool) (string, error) {
	fmt.Fprintf(s.writer, "%s: ", label(name, fallback, secret))

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
