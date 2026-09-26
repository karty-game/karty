// Package uiprototype explores UI binding behavior without defining a public SDK
// or wire protocol. View is a synchronous test double, not the runtime transport.
package uiprototype

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/cartridge"
)

const (
	sectionName    = "karty.experimental.inventory"
	MaxUpdateBytes = 64 * 1024
	MaxRows        = 256
	MaxTextBytes   = 4096
)

var ErrInvalid = errors.New("invalid prototype UI presentation")
var ErrStale = errors.New("obsolete UI action")

// Template is author-owned presentation data, embedded inside a Wasm container.
type Template struct {
	Name   string `json:"name"`
	Title  string `json:"title"`
	Action string `json:"action"`
}

func Load(wasm []byte, name string) (Template, error) {
	data, err := cartridge.ExtractSection(wasm, sectionName)
	if err != nil {
		return Template{}, fmt.Errorf("load UI asset: %w", err)
	}

	var template Template
	if err := json.Unmarshal(data, &template); err != nil {
		return Template{}, fmt.Errorf("decode UI asset: %w", err)
	}

	if template.Name != name || name == "" || template.Action != "use" || len(template.Title) > MaxTextBytes {
		return Template{}, ErrInvalid
	}

	return template, nil
}

// PackageTemplate is experimental packaging, not a production asset format.
func PackageTemplate(wasm, definition []byte) ([]byte, error) {
	result, err := cartridge.EmbedSection(wasm, sectionName, definition)
	if err != nil {
		return nil, fmt.Errorf("package UI asset: %w", err)
	}

	return result, nil
}

// Row IDs must not be reused within an open instance, even after deletion.
type Row struct {
	ID       uint64 `json:"id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

// Update is a prototype operation envelope; Order is explicit when it changes.
type Update struct {
	Instance uint64   `json:"instance"`
	Show     bool     `json:"show"`
	Close    bool     `json:"close"`
	Title    string   `json:"title"`
	Upsert   []Row    `json:"upsert"`
	Remove   []uint64 `json:"remove"`
	Order    []uint64 `json:"order"`
}

// View models atomic application with an explicit acknowledgment. The real Wasm
// import does NOT currently acknowledge commits; runtime integration is pending.
type View struct {
	Instance uint64
	Title    string
	Rows     []Row
	Updates  int
}

func validRows(rows []Row) bool {
	if len(rows) > MaxRows {
		return false
	}

	seen := make(map[uint64]bool, len(rows))
	for _, row := range rows {
		if row.ID == 0 || seen[row.ID] || len(row.Name) > MaxTextBytes || row.Quantity < 0 {
			return false
		}

		seen[row.ID] = true
	}

	return true
}

func (v *View) Apply(update Update) error {
	encoded, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("encode prototype update: %w", err)
	}

	if len(encoded) > MaxUpdateBytes || update.Instance == 0 || len(update.Title) > MaxTextBytes {
		return ErrInvalid
	}

	if update.Close {
		if update.Show || update.Instance != v.Instance || len(update.Upsert)+len(update.Remove)+len(update.Order) != 0 ||
			update.Title != "" {
			return ErrInvalid
		}

		v.Instance, v.Title, v.Rows = 0, "", nil
		v.Updates++

		return nil
	}

	if (!update.Show && update.Instance != v.Instance) || (update.Show && v.Instance != 0) {
		return ErrInvalid
	}

	if !validRows(update.Upsert) {
		return ErrInvalid
	}

	rows := slices.Clone(v.Rows)
	for _, id := range update.Remove {
		index := slices.IndexFunc(rows, func(row Row) bool { return row.ID == id })
		if index < 0 {
			return ErrInvalid
		}

		rows = slices.Delete(rows, index, index+1)
	}

	for _, row := range update.Upsert {
		index := slices.IndexFunc(rows, func(existing Row) bool { return existing.ID == row.ID })
		if index < 0 {
			rows = append(rows, row)
		} else {
			rows[index] = row
		}
	}

	if update.Order != nil {
		if len(update.Order) != len(rows) {
			return ErrInvalid
		}

		ordered := make([]Row, 0, len(rows))
		for _, id := range update.Order {
			index := slices.IndexFunc(rows, func(row Row) bool { return row.ID == id })
			if index < 0 {
				return ErrInvalid
			}

			ordered = append(ordered, rows[index])
		}

		rows = ordered
	}

	if !validRows(rows) {
		return ErrInvalid
	}

	v.Instance, v.Title, v.Rows = update.Instance, update.Title, rows
	v.Updates++

	return nil
}

// Controller owns closures that may access the concrete user *Game. No domain
// struct, reflection, generic collection, or per-widget Game proxy is needed.
type Controller struct {
	instance    uint64
	template    Template
	project     func() []Row
	action      func(uint64) bool
	apply       func(Update) error
	accepted    []Row
	retired     map[uint64]bool
	revision    uint64
	flushed     uint64
	opened      bool
	closed      bool
	Projections int
}

// Session supplies monotonically increasing identities, shared across screens.
type Session struct{ next uint64 }

func (s *Session) Open(template Template, project func() []Row, action func(uint64) bool, apply func(Update) error) (*Controller, error) {
	if s.next == ^uint64(0) || project == nil || action == nil || apply == nil {
		return nil, ErrInvalid
	}

	s.next++

	controller := &Controller{
		instance: s.next,
		template: template,
		project:  project,
		action:   action,
		apply:    apply,
		revision: 1,
		retired:  make(map[uint64]bool),
	}
	if err := controller.Flush(); err != nil {
		return nil, err
	}

	return controller, nil
}

func (c *Controller) Invalidate() {
	if !c.closed {
		c.revision++
	}
}

func (c *Controller) Flush() error {
	if c.closed || c.flushed == c.revision {
		return nil
	}

	revision := c.revision
	rows := slices.Clone(c.project())
	c.Projections++

	if !validRows(rows) {
		return ErrInvalid
	}

	update := Update{Instance: c.instance, Show: !c.opened, Title: c.template.Title}
	order := make([]uint64, 0, len(rows))

	priorOrder := make([]uint64, 0, len(c.accepted))
	for _, row := range rows {
		if c.retired[row.ID] {
			return ErrInvalid
		}

		order = append(order, row.ID)

		index := slices.IndexFunc(c.accepted, func(old Row) bool { return old.ID == row.ID })
		if index < 0 || c.accepted[index] != row {
			update.Upsert = append(update.Upsert, row)
		}
	}

	for _, row := range c.accepted {
		priorOrder = append(priorOrder, row.ID)
		if !slices.Contains(order, row.ID) {
			update.Remove = append(update.Remove, row.ID)
		}
	}

	if !slices.Equal(order, priorOrder) {
		update.Order = order
	}

	if update.Show || len(update.Upsert)+len(update.Remove) != 0 || update.Order != nil {
		if err := c.apply(update); err != nil {
			return fmt.Errorf("apply inventory: %w", err)
		}
	}

	for _, id := range update.Remove {
		c.retired[id] = true
	}

	c.accepted, c.flushed, c.opened = rows, revision, true

	return nil
}

func (c *Controller) Dispatch(instance, rowID uint64, action string) error {
	if c.closed || instance != c.instance || action != c.template.Action {
		return ErrStale
	}

	if !slices.ContainsFunc(c.accepted, func(row Row) bool { return row.ID == rowID }) || !c.action(rowID) {
		return ErrStale
	}

	c.Invalidate()

	return nil
}

func (c *Controller) Close() error {
	if c.closed {
		return nil
	}

	if err := c.apply(Update{Instance: c.instance, Close: true}); err != nil {
		return fmt.Errorf("close inventory: %w", err)
	}

	c.closed = true
	c.project, c.action, c.apply, c.accepted, c.retired = nil, nil, nil, nil, nil

	return nil
}
