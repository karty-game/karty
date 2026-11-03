//go:build ignore

// Regenerate the embedded upstream decoder without downloading or executing JS.
package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"regexp"
	"strconv"
)

func main() {
	source, err := os.ReadFile("upstream.js")
	if err != nil {
		panic(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(source)) != "784315b6959c85459eab77f51776158cc381d7e7d77b919ff7c4935f088a7c5c" {
		panic("upstream Meshopt source checksum changed")
	}
	packed := regexp.MustCompile(`(?s)var wasm_base\s*=\s*'([^']+)'`).FindSubmatch(source)
	table := regexp.MustCompile(`(?s)var wasmpack\s*=\s*new Uint8Array\(\[([^]]+)\]`).FindSubmatch(source)
	if packed == nil || table == nil {
		panic("missing pinned upstream data")
	}
	var dictionary []byte
	for _, number := range regexp.MustCompile(`[0-9]+`).FindAll(table[1], -1) {
		value, err := strconv.ParseUint(string(number), 10, 8)
		if err != nil {
			panic(err)
		}
		dictionary = append(dictionary, byte(value))
	}
	var codes []int
	for _, code := range packed[1] {
		value := int(code) + 4
		if code > 96 {
			value = int(code) - 97
		} else if code > 64 {
			value = int(code) - 39
		}
		codes = append(codes, value)
	}
	var wasm []byte
	for index := 0; index < len(codes); index++ {
		value := codes[index]
		if value < 60 {
			value = int(dictionary[value])
		} else {
			index++
			value = (value-60)*64 + codes[index]
		}
		wasm = append(wasm, byte(value))
	}
	if err := os.WriteFile("decoder.wasm", wasm, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("decoder.wasm: %d bytes, sha256 %x\n", len(wasm), sha256.Sum256(wasm))
}
