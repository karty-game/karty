package project

import "github.com/karty-game/karty-ui/compiler"

// CompileUI adapts project asset configuration to KartUI's independent compiler.
func CompileUI(directory string, entries []Texture, layouts []Layout, theme string) ([]uicompiler.Component, error) {
	sources := make([]uicompiler.Source, len(entries))
	for i, entry := range entries {
		sources[i] = uicompiler.Source{Name: entry.Name, Source: entry.Source}
	}

	layoutSources := make([]uicompiler.LayoutSource, len(layouts))
	for i, entry := range layouts {
		layoutSources[i] = uicompiler.LayoutSource{Source: entry.Source}
	}

	return uicompiler.LoadProjectWithTheme(directory, sources, layoutSources, theme)
}
