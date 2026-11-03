package generator

import "github.com/karty-game/karty/website"

func softwareCapturePage() (guidePage, error) {
	contents, err := website.Files.ReadFile("content/guides/software-capture.md")
	if err != nil {
		return guidePage{}, err
	}

	return guidePage{name: "software-capture", title: "Software captures", markdown: string(contents)}, nil
}
