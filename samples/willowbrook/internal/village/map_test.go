package village

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/karty-game/tiled"
	"github.com/karty-game/tiled/source"
)

func TestCompiledMapRetainsAuthoredGameplayAndSprites(t *testing.T) {
	if !reflect.DeepEqual(Level.Sprites, authoredLevel.Sprites) ||
		!reflect.DeepEqual(Level.Objects, authoredLevel.Objects) ||
		!reflect.DeepEqual(Level.LayerNames, authoredLevel.LayerNames) ||
		!reflect.DeepEqual(Level.Properties, authoredLevel.Properties) {
		t.Fatal("compiled map changed authored sprites or gameplay metadata")
	}
}

func TestAuthoredGameplayAndCollision(t *testing.T) {
	if !Walkable(Spawn) {
		t.Fatal("blocked authored spawn")
	}
	for _, p := range QuestStations {
		if !Walkable(p) {
			t.Fatalf("blocked authored interaction: %+v", p)
		}
	}
	if len(Colliders) < 5 {
		t.Fatal("missing Tiled collision objects")
	}
	for _, r := range Colliders {
		if Walkable(Point{r.X + r.Width/2, r.Y + r.Height/2}) {
			t.Fatalf("walked through %s", r.Name)
		}
	}
	for _, p := range []Point{{-1, 0}, {Width() + 1, 100}, {100, Height() + 1}, {math.NaN(), 100}} {
		if Walkable(p) {
			t.Fatal("escaped map bounds")
		}
	}
}

func TestMapEditsChangeSpritesAndQuestObjectsWithoutGameCode(t *testing.T) {
	raw, e := os.ReadFile("../../" + "internal/village/village.tmj")
	if e != nil {
		t.Fatal(e)
	}
	var document map[string]any
	if e = json.Unmarshal(raw, &document); e != nil {
		t.Fatal(e)
	}
	var movedID int
	for _, value := range document["layers"].([]any) {
		l := value.(map[string]any)
		if l["name"] == "Scenery" {
			o := l["objects"].([]any)[0].(map[string]any)
			movedID = int(o["id"].(float64))
			o["x"] = o["x"].(float64) + 48
		}
		if l["name"] == "Gameplay" {
			for _, v := range l["objects"].([]any) {
				o := v.(map[string]any)
				if o["name"] == "cottage-note" {
					o["x"] = o["x"].(float64) + 16
				}
			}
		}
	}
	changed, e := json.Marshal(document)
	if e != nil {
		t.Fatal(e)
	}
	tsj, e := os.ReadFile("../../" + "internal/village/village.tsj")
	if e != nil {
		t.Fatal(e)
	}
	edited, e := source.Load(
		fstest.MapFS{"internal/village/village.tmj": {Data: changed}, "internal/village/village.tsj": {Data: tsj}},
		"internal/village/village.tmj",
	)
	if e != nil {
		t.Fatal(e)
	}
	originalX, editedX := 0.0, 0.0
	for _, s := range Level.Sprites {
		if s.ObjectID == movedID {
			originalX = s.X
		}
	}
	for _, s := range edited.Sprites {
		if s.ObjectID == movedID {
			editedX = s.X
		}
	}
	if editedX-originalX != 48 {
		t.Fatal("scenery position did not follow Tiled edit")
	}
	for _, o := range edited.Objects {
		if o.Name == "cottage-note" {
			if o.X != QuestStations[0].X+16 {
				t.Fatal("clue position ignored Tiled edit")
			}
			return
		}
	}
	t.Fatal("edited clue disappeared")
}

var authoredLevel *tiled.Map

func init() {
	m, err := source.Load(os.DirFS("../.."), "internal/village/village.tmj")
	if err != nil {
		panic(err)
	}
	authoredLevel = m
	encoded, err := tiled.Encode(m)
	if err != nil {
		panic(err)
	}
	compiled, err := tiled.Decode(encoded)
	if err != nil {
		panic(err)
	}
	Configure(compiled)
}
