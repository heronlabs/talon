package core

import "fmt"

// ConfigReader reads the value of every key of the configuration.
type ConfigReader interface {
	Read() (map[string]string, error)
}

// ConfigWriter writes the configuration to .env.
type ConfigWriter interface {
	Write(values map[string]string) error
}

// SetupService writes the configuration of the host to .env.
type SetupService struct {
	reader ConfigReader
	writer ConfigWriter
}

// NewSetupService returns a SetupService that reads the configuration through reader and writes it through writer.
func NewSetupService(reader ConfigReader, writer ConfigWriter) *SetupService {
	return &SetupService{reader: reader, writer: writer}
}

// Configure reads the configuration and writes it to .env.
func (s *SetupService) Configure() error {
	values, err := s.reader.Read()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if err := s.writer.Write(values); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	return nil
}
