// Package art owns the sample's three ordinary, content-addressed level packs.
// The engine verifies and decodes a complete pack before its ready callback. One extra
// rendered update lets its queued 2D texture uploads run before story use.
package art

import (
	"example.com/last-train/.karty/assets"
	"example.com/last-train/.karty/engine"
)

type asset struct {
	pack string
	id   uint32
}
type pack struct {
	name    engine.LevelName
	request engine.RequestID
	handle  engine.LevelHandle
	settle  uint8
	failure string
}
type Loader struct {
	game    *engine.Game
	packs   map[string]*pack
	onReady func()
}

func New(game *engine.Game, onReady func()) *Loader {
	return &Loader{game: game, onReady: onReady, packs: map[string]*pack{
		"opening":       {name: assets.LevelArtOpening},
		"investigation": {name: assets.LevelArtInvestigation},
		"conclusion":    {name: assets.LevelArtConclusion},
	}}
}
func (l *Loader) Request(names ...string) {
	for _, name := range names {
		p := l.packs[name]
		if p != nil && p.request == 0 && p.handle == 0 && p.failure == "" {
			p.failure = ""
			l.load(p)
		}
	}
}
func (l *Loader) Retry(names ...string) {
	for _, name := range names {
		if p := l.packs[name]; p != nil {
			p.failure = ""
		}
	}
	l.Request(names...)
}
func (l *Loader) Ready(names ...string) bool {
	for _, name := range names {
		p := l.packs[name]
		if p == nil || p.handle == 0 || p.settle != 0 {
			return false
		}
	}
	return true
}
func (l *Loader) Step() {
	for _, p := range l.packs {
		if p.settle > 0 {
			p.settle--
		}
	}
}
func (l *Loader) load(p *pack) {
	var request engine.RequestID
	request = l.game.LoadLevel(p.name, func(event engine.LevelReady) {
		// A cancelled speculative load can win its race with publication.
		if p.request != request {
			l.game.ReleaseLevel(event.Handle)
			return
		}
		p.request, p.handle, p.settle = 0, event.Handle, 2
		if l.onReady != nil {
			l.onReady()
		}
	})
	p.request = request
}
func (l *Loader) Failed(e engine.LevelFailed) {
	for _, p := range l.packs {
		if p.request == e.RequestID {
			p.request = 0
			p.failure = e.Diagnostic
			return
		}
	}
}
func (l *Loader) Failure(names ...string) string {
	for _, name := range names {
		if p := l.packs[name]; p != nil && p.failure != "" {
			return p.failure
		}
	}
	return ""
}
func (l *Loader) CancelConclusion() {
	p := l.packs["conclusion"]
	if p.request != 0 {
		l.game.CancelLevelRequest(p.request)
		p.request = 0
	}
}
func (l *Loader) Close() {
	for _, p := range l.packs {
		if p.request != 0 {
			l.game.CancelLevelRequest(p.request)
			p.request = 0
		}
		if p.handle != 0 {
			l.game.ReleaseLevel(p.handle)
			p.handle = 0
		}
	}
}
func Owner(name string) string { return catalog[name].pack }
func (l *Loader) Texture(name string) engine.LevelTextureRef {
	a, ok := catalog[name]
	if !ok || !l.Ready(a.pack) {
		return engine.LevelTextureRef{}
	}
	return engine.LevelTexture(l.packs[a.pack].handle, a.id)
}
func (l *Loader) Source(name string) engine.UISource {
	a, ok := catalog[name]
	if !ok || !l.Ready(a.pack) {
		return engine.UISource{}
	}
	return engine.UISource{Handle: l.packs[a.pack].handle, Name: engine.UIAsset("art." + name)}
}
