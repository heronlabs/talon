package cms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// Printer writes reports as indented contract JSON; it is the sink of `report --once`.
type Printer struct{ w io.Writer }

// NewPrinter builds a Printer over w.
func NewPrinter(w io.Writer) *Printer { return &Printer{w: w} }

// Publish implements port.ReportSink.
func (p *Printer) Publish(_ context.Context, r domain.Report) error {
	enc := json.NewEncoder(p.w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ToDTO(r)); err != nil {
		return fmt.Errorf("print report: %w", err)
	}
	return nil
}
