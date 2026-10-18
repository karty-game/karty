package generator

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/karty-game/karty/website"
)

type generatedFile struct {
	name, contents string
}

type guideLink struct {
	Name, URL string
}

type pathCard struct {
	Name, URL, Image, Alt, Purpose string
}

type guidePage struct {
	name, title, markdown string
	worldTopics           []guideLink
	cards                 []pathCard
	design                bool
	technical             bool
	redirect              string
}

func pathCards() []pathCard {
	return []pathCard{
		{
			Name:    "2D game",
			URL:     "2d.html",
			Image:   "assets/2d-scene.jpg",
			Alt:     "A 2D sprite beside vector shapes and text",
			Purpose: "Draw sprites, vector shapes, and text. Add movement and game rules.",
		},
		{
			Name:    "3D first person",
			URL:     "fps.html",
			Image:   "assets/hangar.jpg",
			Alt:     "The Hangar viewed from eye height",
			Purpose: "Walk through rooms and interact with nearby objects.",
		},
		{
			Name:    "3D isometric",
			URL:     "isometric.html",
			Image:   "assets/isometric.jpg",
			Alt:     "The Roman court viewed from an isometric camera",
			Purpose: "View rooms from above and orbit around the map.",
		},
	}
}

func worldTopics() []guideLink {
	return []guideLink{
		{Name: "One room", URL: "levels.html"}, {Name: "Textures", URL: "textures.html"},
		{Name: "Prefabs", URL: "prefabs.html"}, {Name: "Lighting", URL: "lighting.html"},
		{Name: "Interactions", URL: "interactions.html"},
	}
}

func commonTopics() []guideLink {
	return []guideLink{
		{Name: "Menus and layouts", URL: "ui.html"}, {Name: "Sound and video", URL: "media.html"},
		{Name: "Build and share", URL: "share.html"},
	}
}

func pageFiles() ([]generatedFile, error) {
	pages, err := guidePages()
	if err != nil {
		return nil, err
	}

	return renderPages(pages)
}

func renderPages(pages []guidePage) ([]generatedFile, error) {
	pageTemplate, err := template.New("page.html").Funcs(template.FuncMap{
		"siteCSS": func() template.CSS {
			return template.CSS(website.CSS()) // #nosec G203 -- stylesheet is trusted, embedded repository source.
		},
	}).ParseFS(website.Files, "templates/page.html")
	if err != nil {
		return nil, err
	}

	files := make([]generatedFile, 0, len(pages)*2)
	for _, page := range pages {
		body, sections, err := renderMarkdown(page.markdown)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", page.name, err)
		}

		markdownName := page.name + ".md"
		if page.name == "complete" {
			markdownName = "guide.md"
		}

		var output strings.Builder
		if err := pageTemplate.Execute(&output, struct {
			Title, Markdown, Current string
			Design                   bool
			Technical                bool
			Guide                    bool
			Redirect                 string
			Body                     template.HTML
			Sections                 []section
			Paths                    []pathCard
			WorldTopics              []guideLink
			SharedTopics             []guideLink
			DesignTopics             []guideLink
			TechnicalTopics          []guideLink
			Cards                    []pathCard
		}{
			Title:           page.title,
			Markdown:        markdownName,
			Current:         page.name + ".html",
			Design:          page.design,
			Technical:       page.technical,
			Guide:           !page.design && !page.technical,
			Redirect:        page.redirect,
			Body:            template.HTML(body), // #nosec G203 -- renderer escapes text and validates links; raw HTML is never accepted.
			Sections:        sections,
			Paths:           pathCards(),
			WorldTopics:     page.worldTopics,
			SharedTopics:    commonTopics(),
			DesignTopics:    designTopics(),
			TechnicalTopics: technicalTopics(),
			Cards:           page.cards,
		}); err != nil {
			return nil, err
		}

		files = append(files, generatedFile{name: page.name + ".html", contents: output.String()},
			generatedFile{name: markdownName, contents: page.markdown})
	}

	return files, nil
}

func guidePages() ([]guidePage, error) {
	landing, err := website.Files.ReadFile("content/guides/index.md")
	if err != nil {
		return nil, err
	}

	pages := []guidePage{
		{name: "index", title: "Choose your Karty game", markdown: string(landing), cards: pathCards()},
	}
	for _, track := range []struct{ name, title string }{
		{"2d", "Build a 2D game"}, {"fps", "Make a first-person game"}, {"isometric", "Make an isometric game"},
	} {
		contents, err := website.Files.ReadFile("content/guides/tracks/" + track.name + ".md")
		if err != nil {
			return nil, err
		}

		var topics []guideLink
		if track.name != "2d" {
			topics = worldTopics()
		}

		pages = append(
			pages,
			guidePage{name: track.name, title: track.title, markdown: pageHeadings(string(contents)), worldTopics: topics},
		)
	}

	for _, topic := range []struct{ name, chapter string }{
		{"levels", "02-level.md"}, {"textures", "03-texture.md"}, {"prefabs", "04-prefab.md"}, {"lighting", "05-light.md"},
		{"interactions", "06-hooks.md"}, {"media", "07-media.md"}, {"ui", "08-ui.md"}, {"share", "09-share.md"},
	} {
		contents, err := website.Files.ReadFile("content/guides/chapters/" + topic.chapter)
		if err != nil {
			return nil, err
		}

		markdown := pageHeadings(string(contents))
		title, _, _ := strings.Cut(strings.TrimPrefix(markdown, "# "), "\n")

		topics := worldTopics()
		if topic.name == "ui" || topic.name == "media" || topic.name == "share" {
			topics = nil
		}

		pages = append(
			pages,
			guidePage{name: topic.name, title: title, markdown: markdown, worldTopics: topics},
		)
	}

	complete, err := readMarkdown()
	if err != nil {
		return nil, err
	}

	pages = append(
		pages,
		guidePage{name: "complete", title: "Make a game with Karty", markdown: complete, worldTopics: worldTopics()},
	)

	// Preserve old bookmarks and Markdown downloads after moving this explanation.
	why, err := website.Files.ReadFile("content/design/why.md")
	if err != nil {
		return nil, err
	}

	legacyWhy := string(why)
	for _, topic := range designTopics() {
		legacyWhy = strings.ReplaceAll(legacyWhy, "]("+topic.URL+")", "](../design/"+topic.URL+")")
	}

	pages = append(pages, guidePage{
		name: "why", title: "Why Karty?", markdown: legacyWhy, redirect: "../design/why.html",
	})

	return pages, nil
}

func designTopics() []guideLink {
	return []guideLink{
		{Name: "Why Karty?", URL: "why.html"},
		{Name: "Architecture", URL: "architecture.html"},
		{Name: "SDK versions and toolchains", URL: "sdk-and-toolchain.html"},
		{Name: "Asset pipeline", URL: "asset-pipeline.html"},
		{Name: "Packages", URL: "packages.html"},
	}
}

func designPages() ([]guidePage, error) {
	pages := make([]guidePage, 0, len(designTopics())+1)

	for _, name := range []string{"index", "why", "architecture", "sdk-and-toolchain", "asset-pipeline", "packages"} {
		contents, err := website.Files.ReadFile("content/design/" + name + ".md")
		if err != nil {
			return nil, err
		}

		markdown := string(contents)
		title, _, _ := strings.Cut(strings.TrimPrefix(markdown, "# "), "\n")
		pages = append(pages, guidePage{name: name, title: title, markdown: markdown, design: true})
	}

	return pages, nil
}

func pageHeadings(markdown string) string {
	if strings.HasPrefix(markdown, "## ") {
		markdown = strings.TrimPrefix(markdown, "#")
		markdown = strings.ReplaceAll(markdown, "\n### ", "\n## ")
	}

	return markdown
}
