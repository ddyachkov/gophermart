package random

// copied from "github.com/Yandex-Practicum/go-autotests/internal/random"

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestASCIIString(t *testing.T) {
	generated := make(map[string]struct{})

	for i := 0; i < 5000; i++ {
		str := ASCIIString(5, 15)
		require.NotContains(t, generated, str)
		generated[str] = struct{}{}
	}
}
