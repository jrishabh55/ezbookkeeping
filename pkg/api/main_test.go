package api

import (
	"os"
	"testing"
)

// TestMain runs from the repository root so templates load as they do at runtime.
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
