// Copyright 2016 Zack Guo <gizak@icloud.com>. All rights reserved.
// Use of this source code is governed by a MIT license that can
// be found in the LICENSE file.

package termui

import (
	"context"
	"image"
	"sync"
	"time"

	tm "github.com/nsf/termbox-go"
)

// Bufferer should be implemented by all renderable components.
type Bufferer interface {
	Buffer() Buffer
}

// Init initializes termui library. This function should be called before any others.
// After initialization, the library must be finalized by 'Close' function.
func Init() error {
	if err := tm.Init(); err != nil {
		return err
	}

	sysEvtChs = make([]chan Event, 0)
	go hookTermboxEvt()

	renderJobs = make(chan []Bufferer)

	Body = NewGrid()
	Body.X = 0
	Body.Y = 0
	Body.BgColor = ThemeAttr("bg")
	Body.Width = TermWidth()

	DefaultEvtStream.Init()
	DefaultEvtStream.Merge("termbox", NewSysEvtCh())
	DefaultEvtStream.Merge("timer", NewTimerCh(time.Second))
	DefaultEvtStream.Merge("custom", usrEvtCh)

	DefaultEvtStream.Handle("/", DefualtHandler)
	DefaultEvtStream.Handle("/sys/wnd/resize", func(e Event) {
		w := e.Data.(EvtWnd)
		Body.Width = w.Width
	})

	DefaultWgtMgr = NewWgtMgr()
	DefaultEvtStream.Hook(DefaultWgtMgr.WgtHandlersHook())

	go func() {
		for bs := range renderJobs {
			render(bs...)
		}
	}()

	return nil

}

// Close finalizes termui library,
// should be called after successful initialization when termui's functionality isn't required anymore.
func Close() {
	once.Do(func() {
		Defer(tm.Close)
	})
}

var renderLock sync.Mutex
var once sync.Once

func termSync() (int, int) {
	tm.Sync()
	termWidth, termHeight = tm.Size()
	return termWidth, termHeight
}

// TermWidth returns the current terminal's width.
func TermWidth() int {
	termSync()
	return termWidth
}

// TermHeight returns the current terminal's height.
func TermHeight() int {
	termSync()
	return termHeight
}

// Render renders all Bufferer in the given order from left to right,
// right could overlap on left ones.
func render(bs ...Bufferer) {
	for _, b := range bs {

		buf := b.Buffer()
		// set cels in buf
		for p, c := range buf.CellMap {
			if p.In(buf.Area) {

				tm.SetCell(p.X, p.Y, c.Ch, toTmAttr(c.Fg), toTmAttr(c.Bg))

			}
		}

	}

	// render
	Defer(func() {
		tm.Flush()
	})
}

func Clear() {
	Defer(func() {
		tm.Clear(tm.ColorDefault, toTmAttr(ThemeAttr("bg")))
	})
}

func clearArea(r image.Rectangle, bg Attribute) {
	for i := r.Min.X; i < r.Max.X; i++ {
		for j := r.Min.Y; j < r.Max.Y; j++ {
			tm.SetCell(i, j, ' ', tm.ColorDefault, toTmAttr(bg))
		}
	}
}

func ClearArea(r image.Rectangle, bg Attribute) {
	Defer(func() {
		clearArea(r, bg)
		tm.Flush()
	})
}

var renderJobs chan []Bufferer

func Render(bs ...Bufferer) {
	//go func() { renderJobs <- bs }()
	//	renderJobs <- bs
	for _, b := range bs {
		render(b)
	}
}

var (
	worker = NewWorker(context.Background())
)

type workerFunc func()
type deferredWorker struct {
	workerChan chan workerFunc
	ctx        context.Context
}

func (d *deferredWorker) loop() {
	for {
		select {
		case <-d.ctx.Done():
			return
		case wf := <-d.workerChan:
			wf()
		}
	}
}

func Defer(wf workerFunc) {
	worker.workerChan <- wf
}

func NewWorker(ctx context.Context) *deferredWorker {
	d := deferredWorker{
		workerChan: make(chan workerFunc),
		ctx:        ctx,
	}
	go d.loop()
	return &d
}
