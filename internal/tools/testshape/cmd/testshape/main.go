// Command testshape runs the testshape analyzer.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/heronlabs/vps-agent/internal/tools/testshape"
)

func main() { singlechecker.Main(testshape.Analyzer) }
