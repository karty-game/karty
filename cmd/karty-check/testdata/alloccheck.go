//go:build karty_alloccheck

package main

import "runtime"

// Diagnostic-only exports injected into the temporary cartridge fixture.
// Ordinary sample and generated projects never contain this instrumentation.
var allocationStats runtime.MemStats

//go:wasmexport karty_allocations
func guestAllocations() uint64 {
	runtime.ReadMemStats(&allocationStats)

	return allocationStats.Mallocs
}

//go:wasmexport karty_allocated_bytes
func guestAllocatedBytes() uint64 {
	runtime.ReadMemStats(&allocationStats)

	return allocationStats.TotalAlloc
}
