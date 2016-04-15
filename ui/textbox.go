// Copyright © 2016, The T Authors.

package ui

import (
	"errors"
	"image"
	"net/url"
	"path"
	"sync"

	"github.com/eaburns/T/edit"
	"github.com/eaburns/T/editor"
	"github.com/eaburns/T/editor/view"
	"github.com/eaburns/T/ui/text"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
)

// A textBox is an editable text box.
type textBox struct {
	bufferURL *url.URL
	view      *view.View
	opts      text.Options
	setter    *text.Setter
	text      *text.Text

	mu    sync.RWMutex
	reset bool
	win   *window
}

// NewTextBod creates a new text box.
// URL is either the root path to an editor server,
// or the path to an open buffer of an editor server.
func newTextBox(w *window, URL url.URL, style text.Style) (t *textBox, err error) {
	if URL.Path == "/" {
		URL.Path = path.Join("/", "buffers")
		buf, err := editor.NewBuffer(&URL)
		if err != nil {
			return nil, err
		}
		URL.Path = buf.Path
		defer func(newBufferURL url.URL) {
			if err != nil {
				editor.Close(&newBufferURL)
			}
		}(URL)
	}
	if ok, err := path.Match("/buffer/*", URL.Path); err != nil {
		// The only error is path.ErrBadPattern. This pattern is not bad.
		panic(err)
	} else if !ok {
		return nil, errors.New("bad buffer path: " + URL.Path)
	}

	v, err := view.New(&URL, '.')
	if err != nil {
		return nil, err
	}
	opts := text.Options{
		DefaultStyle: style,
		TabWidth:     4,
		Padding:      2,
	}
	setter := text.NewSetter(opts)
	t = &textBox{
		bufferURL: &URL,
		view:      v,
		opts:      opts,
		setter:    setter,
		text:      setter.Set(),
		win:       w,
	}
	go func() {
		for range v.Notify {
			t.mu.Lock()
			t.reset = true
			if t.win != nil {
				t.win.Send(paint.Event{})
			}
			t.mu.Unlock()
		}
	}()
	return t, nil
}

func (t *textBox) close() {
	t.mu.Lock()
	t.win = nil
	t.mu.Unlock()

	t.text.Release()
	t.setter.Release()
	t.view.Close()
	editor.Close(t.bufferURL)
}

// SetSize resets the text if either the size changed or the text changed.
func (t *textBox) setSize(size image.Point) {
	t.mu.Lock()
	if !t.reset && t.opts.Size == size {
		t.mu.Unlock()
		return
	}
	t.reset = false
	t.mu.Unlock()

	h := t.opts.DefaultStyle.Face.Metrics().Height
	t.view.Resize(size.Y / int(h>>6))
	t.text.Release()
	t.opts.Size = size
	t.setter.Reset(t.opts)
	t.view.View(func(text []byte, _ []view.Mark) { t.setter.Add(text) })
	t.text = t.setter.Set()
}

func (t *textBox) draw(pt image.Point, scr screen.Screen, win screen.Window) {
	t.text.Draw(pt, scr, win)
}

func (t *textBox) drawLines(pt image.Point, scr screen.Screen, win screen.Window) {
	t.text.DrawLines(pt, scr, win)
}

var (
	advanceDot = edit.Set(edit.Dot.Plus(edit.Clamp(edit.Rune(1))), '.')
	backspace  = edit.Delete(edit.Dot.Minus(edit.Clamp(edit.Rune(1))).To(edit.Dot))
	newline    = []edit.Edit{edit.Change(edit.Dot, "\n"), advanceDot}
	tab        = []edit.Edit{edit.Change(edit.Dot, "\t"), advanceDot}
)

func (t *textBox) key(w *window, event key.Event) bool {
	if event.Direction == key.DirRelease {
		return false
	}
	switch event.Code {
	case key.CodeDeleteBackspace:
		t.view.Do(nil, backspace)
	case key.CodeReturnEnter:
		t.view.Do(nil, newline...)
	case key.CodeTab:
		t.view.Do(nil, tab...)
	default:
		if event.Rune >= 0 {
			t.view.Do(nil, edit.Change(edit.Dot, string(event.Rune)), advanceDot)
		}
	}
	return false
}

func (t *textBox) mouse(*window, mouse.Event) bool               { return false }
func (t *textBox) drawLast(scr screen.Screen, win screen.Window) {}