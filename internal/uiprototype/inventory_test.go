package uiprototype

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestPackagedTemplate(t *testing.T) {
	t.Parallel()

	wasm, err := PackageTemplate([]byte("\x00asm\x01\x00\x00\x00"), []byte(`{"name":"ui.inventory","title":"Inventory","action":"use"}`))
	if err != nil {
		t.Fatal(err)
	}

	template, err := Load(wasm, "ui.inventory")
	if err != nil || template.Title != "Inventory" {
		t.Fatalf("load: %+v, %v", template, err)
	}

	if _, err := Load(wasm, "missing"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing name: %v", err)
	}

	if _, err := Load([]byte("not wasm"), "ui.inventory"); err == nil {
		t.Fatal("accepted invalid container")
	}
}

func TestInventoryLifecycle(t *testing.T) {
	t.Parallel()

	rows := []Row{{ID: 1, Name: "Potion", Quantity: 3}, {ID: 2, Name: "Key", Quantity: 1}}

	var (
		session Session
		view    View
		updates []Update
	)

	apply := func(update Update) error {
		updates = append(updates, update)

		return view.Apply(update)
	}

	controller, err := session.Open(Template{Action: "use"}, func() []Row { return rows }, func(id uint64) bool {
		for index := range rows {
			if rows[index].ID == id && rows[index].Quantity > 0 {
				rows[index].Quantity--

				return true
			}
		}

		return false
	}, apply)
	if err != nil {
		t.Fatal(err)
	}

	instance := view.Instance
	for range 2 {
		if err := controller.Dispatch(instance, 1, "use"); err != nil {
			t.Fatal(err)
		}
	}

	if view.Rows[0].Quantity != 3 {
		t.Fatal("view aliases domain data")
	}

	if err := controller.Flush(); err != nil {
		t.Fatal(err)
	}

	if controller.Projections != 2 || view.Updates != 2 || view.Rows[0].Quantity != 1 || len(updates[1].Upsert) != 1 {
		t.Fatal("actions did not coalesce into one row patch")
	}

	if err := controller.Flush(); err != nil {
		t.Fatal(err)
	}

	if controller.Projections != 2 || view.Updates != 2 {
		t.Fatal("idle frame did work")
	}

	controller.Invalidate()

	if err := controller.Flush(); err != nil {
		t.Fatal(err)
	}

	if view.Updates != 2 {
		t.Fatal("unchanged projection emitted update")
	}

	rows[0], rows[1] = rows[1], rows[0]

	controller.Invalidate()

	if err := controller.Flush(); err != nil {
		t.Fatal(err)
	}

	if view.Rows[0].ID != 2 || len(updates[2].Upsert) != 0 {
		t.Fatal("reorder was not explicit")
	}

	rows = rows[:1]

	controller.Invalidate()

	if err := controller.Flush(); err != nil {
		t.Fatal(err)
	}

	if err := controller.Dispatch(instance, 1, "use"); !errors.Is(err, ErrStale) {
		t.Fatalf("removed row action: %v", err)
	}

	rows = append(rows, Row{ID: 1, Name: "Reused key"})

	controller.Invalidate()

	if err := controller.Flush(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("reused key: %v", err)
	}

	if err := controller.Close(); err != nil {
		t.Fatal(err)
	}

	if controller.project != nil || controller.action != nil || controller.apply != nil {
		t.Fatal("close retained callbacks")
	}

	if err := controller.Dispatch(instance, 2, "use"); !errors.Is(err, ErrStale) {
		t.Fatal("closed controller accepted action")
	}

	checkRepeatedScreens(t, &session, &view, instance)
}

func checkRepeatedScreens(t *testing.T, session *Session, view *View, instance uint64) {
	t.Helper()

	for range 10 {
		next, err := session.Open(Template{Action: "use"}, func() []Row { return nil }, func(uint64) bool { return true }, view.Apply)
		if err != nil {
			t.Fatal(err)
		}

		if view.Instance <= instance {
			t.Fatal("reused instance identity")
		}

		if err := next.Dispatch(instance, 2, "use"); !errors.Is(err, ErrStale) {
			t.Fatal("replacement accepted stale action")
		}

		instance = view.Instance

		if err := next.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRejectedUpdateAndClose(t *testing.T) {
	t.Parallel()

	rows := []Row{{ID: 1, Name: "Potion", Quantity: 3}}

	var (
		session Session
		view    View
	)

	reject := false

	controller, err := session.Open(
		Template{},
		func() []Row { return rows },
		func(uint64) bool { return false },
		func(update Update) error {
			if reject {
				return ErrInvalid
			}

			return view.Apply(update)
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	rows[0].Quantity--

	controller.Invalidate()

	reject = true

	if err := controller.Flush(); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected rejection")
	}

	if controller.accepted[0].Quantity != 3 || view.Rows[0].Quantity != 3 || rows[0].Quantity != 2 {
		t.Fatal("rejection changed baseline or rolled back domain")
	}

	if err := controller.Close(); !errors.Is(err, ErrInvalid) || controller.closed {
		t.Fatal("rejected close disposed controller")
	}

	reject = false

	if err := controller.Flush(); err != nil {
		t.Fatal(err)
	}

	if view.Rows[0].Quantity != 2 {
		t.Fatal("retry lost update")
	}

	if err := controller.Close(); err != nil {
		t.Fatal(err)
	}

	if view.Instance != 0 {
		t.Fatal("close retry failed")
	}
}

func TestAtomicValidationAndAggregateBudget(t *testing.T) {
	t.Parallel()

	view := View{}

	initial := Update{Instance: 1, Show: true, Upsert: []Row{{ID: 1, Name: "old"}}, Order: []uint64{1}}
	if err := view.Apply(initial); err != nil {
		t.Fatal(err)
	}

	before := View{Instance: view.Instance, Title: view.Title, Rows: append([]Row(nil), view.Rows...), Updates: view.Updates}

	oversized := Update{Instance: 1}
	for index := range MaxRows {
		oversized.Upsert = append(oversized.Upsert, Row{ID: uint64(index + 1), Name: strings.Repeat("x", MaxTextBytes)})
	}

	invalid := []Update{
		{Instance: 1, Upsert: []Row{{ID: 1, Name: "new"}}, Order: []uint64{2}},
		{Instance: 1, Upsert: []Row{{ID: 2}, {ID: 2}}},
		{Instance: 1, Upsert: []Row{{ID: 2}}, Order: []uint64{1, 1}},
		{Instance: 1, Remove: []uint64{1, 1}},
		{Instance: 1, Close: true, Upsert: []Row{{ID: 2}}},
		{Instance: 2, Close: true},
		oversized,
	}
	for _, update := range invalid {
		if err := view.Apply(update); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid update accepted: %v", err)
		}

		if !reflect.DeepEqual(view, before) {
			t.Fatal("rejected update partially mutated view")
		}
	}
}

func TestInvalidationDuringProjectionSurvives(t *testing.T) {
	t.Parallel()

	var (
		session    Session
		view       View
		controller *Controller
	)

	project := func() []Row {
		if controller != nil {
			controller.Invalidate()
		}

		return nil
	}

	var err error

	controller, err = session.Open(Template{}, project, func(uint64) bool { return false }, view.Apply)
	if err != nil {
		t.Fatal(err)
	}

	controller.Invalidate()

	if err := controller.Flush(); err != nil {
		t.Fatal(err)
	}

	if controller.flushed == controller.revision {
		t.Fatal("lost invalidation during projection")
	}
}

//nolint:paralleltest // Go forbids AllocsPerRun during parallel tests.
func TestIdleAllocations(t *testing.T) {
	// AllocsPerRun requires a nonparallel test.
	var (
		session Session
		view    View
	)

	controller, err := session.Open(Template{}, func() []Row { return nil }, func(uint64) bool { return false }, view.Apply)
	if err != nil {
		t.Fatal(err)
	}

	allocations := testing.AllocsPerRun(100, func() {
		if err := controller.Flush(); err != nil {
			panic(err)
		}
	})
	if allocations != 0 {
		t.Fatalf("idle allocations: %v", allocations)
	}
}
