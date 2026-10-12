// Command karty-guide builds the public game-making guide without an SDK or network access.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karty-game/karty/internal/guide"
)

func main() {
	output := flag.String("output", "dist/guide", "directory for index.html and guide.md")

	flag.Parse()

	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Usage: karty-guide [-output directory]")
		os.Exit(2)
	}

	if err := guide.Generate(*output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Fprintf(
		os.Stdout,
		"Guide written to %s\nPaths: 2D, first person, isometric\nComplete Markdown: %s\n",
		filepath.Join(*output, "index.html"),
		filepath.Join(*output, "guide.md"),
	)
}
