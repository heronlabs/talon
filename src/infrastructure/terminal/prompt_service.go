package terminal

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type Prompt interface {
	Ask(name, fallback string, secret bool) (string, error)
}

type PromptService struct {
	scanner *bufio.Scanner
	writer  io.Writer
}

func NewPromptService(reader io.Reader, writer io.Writer) *PromptService {
	return &PromptService{scanner: bufio.NewScanner(reader), writer: writer}
}

func (s *PromptService) Ask(name, fallback string, secret bool) (string, error) {
	label := name
	if fallback != "" {
		shown := fallback
		if secret {
			shown = "****"
		}

		label = fmt.Sprintf("%s [%s]", name, shown)
	}
	fmt.Fprintf(s.writer, "%s: ", label)

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
