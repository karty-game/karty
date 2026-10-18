// Command karty-guide builds public guides and design pages without an SDK or network access.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karty-game/karty/website/internal/generator"
)

func main() {
	output := flag.String("output", "dist/guide", "directory for index.html and guide.md")
	designOutput := flag.String("design-output", "", "optional sibling directory for design pages")
	technicalOutput := flag.String("technical-output", "", "optional sibling directory for API pages")

	flag.Parse()

	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Usage: karty-guide [-output directory] [-design-output directory] [-technical-output directory]")
		os.Exit(2)
	}

	if err := generate(*output, *designOutput, *technicalOutput); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *designOutput != "" {
		fmt.Fprintf(os.Stdout, "Design written to %s\n", filepath.Join(*designOutput, "index.html"))
	}

	if *technicalOutput != "" {
		fmt.Fprintf(os.Stdout, "API reference written to %s\n", filepath.Join(*technicalOutput, "index.html"))
	}

	fmt.Fprintf(
		os.Stdout,
		"Guide written to %s\nPaths: 2D, first person, isometric\nComplete Markdown: %s\n",
		filepath.Join(*output, "index.html"),
		filepath.Join(*output, "guide.md"),
	)
}

func generate(output, designOutput, technicalOutput string) error {
	for _, section := range []struct{ name, path string }{{"design", designOutput}, {"technical", technicalOutput}} {
		if section.path == "" {
			continue
		}

		guidePath, err := filepath.Abs(output)
		if err != nil {
			return err
		}

		sectionPath, err := filepath.Abs(section.path)
		if err != nil {
			return err
		}

		if filepath.Base(guidePath) != "guide" || filepath.Base(sectionPath) != section.name ||
			filepath.Dir(guidePath) != filepath.Dir(sectionPath) {
			return fmt.Errorf("combined outputs must be siblings named guide and %s: %w", section.name, os.ErrInvalid)
		}
	}

	if err := generator.Generate(output); err != nil {
		return err
	}

	if designOutput != "" {
		if err := generator.GenerateDesign(designOutput); err != nil {
			return err
		}
	}

	if technicalOutput != "" {
		return generator.GenerateTechnical(technicalOutput)
	}

	return nil
}
