package domain

import (
	"testing"

	"github.com/cockroachdb/apd/v3"
)

func withDefaultContext(t *testing.T, ctx *apd.Context, fn func()) {
	t.Helper()

	original := DefaultContext
	DefaultContext = ctx

	t.Cleanup(func() {
		DefaultContext = original
	})

	fn()
}
