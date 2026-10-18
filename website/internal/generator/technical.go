package generator

import (
	"strings"

	"github.com/karty-game/karty/website"
)

func technicalTopics() []guideLink {
	return []guideLink{
		{Name: "Runtime", URL: "runtime.html"},
		{Name: "Input", URL: "input.html"},
		{Name: "Transforms", URL: "transforms.html"},
		{Name: "Cameras", URL: "cameras.html"},
		{Name: "Sprites and particles", URL: "graphics.html"},
		{Name: "Audio", URL: "audio.html"},
	}
}

func technicalPages() ([]guidePage, error) {
	topics := technicalTopics()
	names := make([]string, 1, len(topics)+1)
	names[0] = "index"

	for _, topic := range topics {
		names = append(names, strings.TrimSuffix(topic.URL, ".html"))
	}

	pages := make([]guidePage, 0, len(names))
	for _, name := range names {
		contents, err := website.Files.ReadFile("content/api/" + name + ".md")
		if err != nil {
			return nil, err
		}

		markdown := string(contents)
		title, _, _ := strings.Cut(strings.TrimPrefix(markdown, "# "), "\n")
		pages = append(pages, guidePage{name: name, title: title, markdown: markdown, technical: true})
	}

	return append(pages, technicalRedirects()...), nil
}

func technicalRedirects() []guidePage {
	pages := make([]guidePage, 0, 9)
	for _, moved := range []struct{ name, title, target string }{
		{"game-lifecycle", "Game lifecycle", "runtime.html#game-lifecycle"},
		{"entities", "Entities and components", "runtime.html#entities-and-components"},
		{"levels", "Levels and resources", "runtime.html#levels-and-resources"},
		{"transform-2d", "Transform 2D", "transforms.html#transform-2d"},
		{"transform-3d", "Transform 3D", "transforms.html#transform-3d"},
		{"camera-2d", "Camera 2D", "cameras.html#camera-2d"},
		{"camera-3d", "Camera 3D", "cameras.html#camera-3d"},
		{"sprites", "Sprites", "graphics.html#sprites"},
		{"particles", "Particles", "graphics.html#particles"},
	} {
		pages = append(pages, guidePage{
			name: moved.name, title: moved.title, technical: true, redirect: moved.target,
			markdown: "# " + moved.title + "\n\nContinue to [" + moved.title + "](" + moved.target + ").\n",
		})
	}

	return pages
}
