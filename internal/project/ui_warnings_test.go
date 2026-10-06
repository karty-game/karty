package project

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	uicompiler "github.com/karty-game/karty-ui/compiler"
	ui "github.com/karty-game/karty-ui/schema"
)

func captureStyleWarningLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	previous := slog.Default()

	t.Cleanup(func() { slog.SetDefault(previous) })

	output := &bytes.Buffer{}
	slog.SetDefault(slog.New(slog.NewTextHandler(output, nil)))

	return output
}

//nolint:paralleltest // These tests replace the process-wide structured logger.
func TestBuildReportsIgnoredStyleWarningsOnce(t *testing.T) {
	// Released public compiler pins have no diagnostic method until the KartUI release.
	component, err := uicompiler.Compile(
		"menu.kui",
		[]byte(`<template>
<panel><label>Text</label></panel>
</template>

<style>
panel
  width: 9999px
  gap: 8
</style>`),
	)
	if _, ok := any(&component).(interface{ StyleWarnings() []string }); !ok {
		t.Skip("requires candidate KartUI diagnostics")
	}

	if err != nil {
		t.Fatal(err)
	}

	output := captureStyleWarningLog(t)

	ReportUIWarnings([]uicompiler.Component{component, component})

	text := output.String()
	if strings.Count(text, "level=WARN") != 1 || !strings.Contains(text, "menu.kui:7:3") || !strings.Contains(text, "width") ||
		!strings.Contains(text, "9999px") {
		t.Fatalf("build warnings: %s", text)
	}
}

//nolint:paralleltest // These tests replace the process-wide structured logger.
func TestStaticLevelReportsStyleWarningsAndKeepsBindingValidation(t *testing.T) {
	component, _ := uicompiler.Compile("probe.kui", []byte(`<template>
<panel><label>Text</label></panel>
</template>`))
	if _, ok := any(&component).(interface{ StyleWarnings() []string }); !ok {
		t.Skip("requires candidate KartUI diagnostics")
	}

	output := captureStyleWarningLog(t)

	template, err := DecodeUIWithTheme(
		"level.kui",
		[]byte(`<template>
<panel><label>Text</label></panel>
</template>

<style>
panel
  width: 9999px
  gap: 8
</style>`),
		uicompiler.DefaultTheme(),
	)
	if err != nil || template.Panel.Gap != 8 || !strings.Contains(output.String(), "level.kui:7:3") ||
		!strings.Contains(output.String(), "width") {
		t.Fatalf("static level styles: %v %+v %s", err, template.Panel, output.String())
	}

	_, err = DecodeUIWithTheme(
		"level.kui",
		[]byte(`<template>
<panel><label>{Title}</label></panel>
</template>

<script setup lang="go">
func setup(Title string) {

}
</script>`),
		uicompiler.DefaultTheme(),
	)
	if !errors.Is(err, ui.ErrTemplate) {
		t.Fatal("dynamic level template was accepted")
	}
}
