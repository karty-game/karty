// karty-ui-prototype exercises a behavioral UI model, not Ebiten or the Wasm ABI.
package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/karty-game/karty/internal/uiprototype"
)

//go:embed inventory.json
var inventoryDefinition []byte

type game struct{ inventory []uiprototype.Row }

func (g *game) use(id uint64) bool {
	for index := range g.inventory {
		item := &g.inventory[index]
		if item.ID == id && item.Quantity > 0 {
			item.Quantity--

			return true
		}
	}

	return false
}

func run() error {
	wasm, err := uiprototype.PackageTemplate([]byte("\x00asm\x01\x00\x00\x00"), inventoryDefinition)
	if err != nil {
		return fmt.Errorf("package: %w", err)
	}

	template, err := uiprototype.Load(wasm, "ui.inventory")
	if err != nil {
		return fmt.Errorf("load: %w", err)
	}

	const initialQuantity = 3

	state := &game{inventory: []uiprototype.Row{{ID: 1, Name: "Potion", Quantity: initialQuantity}}}

	var (
		session uiprototype.Session
		view    uiprototype.View
	)

	controller, err := session.Open(template, func() []uiprototype.Row { return state.inventory }, state.use, view.Apply)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Opened packaged %s: %v\n", template.Name, view.Rows)

	for range 2 {
		if err := controller.Dispatch(view.Instance, 1, "use"); err != nil {
			return fmt.Errorf("action: %w", err)
		}
	}

	if err := controller.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Two actions, one refresh: %v (projections=%d, updates=%d)\n", view.Rows, controller.Projections, view.Updates)

	if err := controller.Flush(); err != nil {
		return fmt.Errorf("idle: %w", err)
	}

	if err := controller.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}

	fmt.Fprintln(os.Stdout, "Idle produced no refresh; closed and disposed controller.")

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
