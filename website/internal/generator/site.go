package generator

import (
	"path/filepath"
	"strings"
)

// GenerateSiteDocs writes all documentation sections with one shared asset
// directory. Relative URLs keep the site usable beneath any hosting prefix.
// Standalone section generators continue to include their own portable assets.
func GenerateSiteDocs(output string) error {
	for _, section := range []struct {
		name  string
		pages func() ([]guidePage, error)
	}{{"guide", guidePages}, {"design", designPages}, {"technical", technicalPages}} {
		pages, err := section.pages()
		if err != nil {
			return err
		}

		files, err := renderPages(pages)
		if err != nil {
			return err
		}

		for i := range files {
			files[i].contents = strings.NewReplacer(`src="assets/`, `src="../assets/`, `href="assets/`, `href="../assets/`, `(assets/`, `(../assets/`).
				Replace(files[i].contents)
		}

		if err := generateFilesWithAssets(filepath.Join(output, section.name), files, false); err != nil {
			return err
		}
	}

	return generateFiles(output, nil)
}
