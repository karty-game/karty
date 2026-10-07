package main

import (
	"os"
	"strings"
	"testing"
)

func TestRuntimeProbeDoesNotOwnContributorInstrumentation(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("testdata/runtime-probe.go")
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(contents), "karty_alloccheck") || strings.Contains(string(contents), "karty_allocations") {
		t.Fatal("runtime probe contains contributor-only allocation instrumentation")
	}
}
