package domain_test

import (
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

func TestBytesOf_KeepsPositiveSizes(t *testing.T) {
	got := domain.BytesOf(800)

	testkit.Equal(t, domain.Bytes(800), got)
}

func TestBytesOf_TreatsNegativeAsZero(t *testing.T) {
	got := domain.BytesOf(-1)

	testkit.Equal(t, domain.Bytes(0), got)
}
