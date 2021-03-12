package main

import (
	"image"
	"log"
	"path/filepath"

	"github.com/fhs/edward/internal/draw"
	"github.com/fhs/edward/internal/frame"
)

var (
	Lheader = []rune("New Cut Paste Snarf Sort Zerox Delcol ")
)

type Column struct {
	row     *Row
	w       []*Window // These are sorted from top to bottom (increasing Y)
	safe    bool
	fortest bool // True if running in test mode (to elide hard to mock actions.)
}

// nw returns the number of Window pointers in Column c.
// TODO(rjk): Consider that this helper is not particularly useful. len is handy.
func (c *Column) nw() int {
	return len(c.w)
}

// Init initializes a new Column object filling image r and drawn to
// display dis.
// TODO(rjk): Why does this need to handle the case where c is nil?
// TODO(rjk): Do we (re)initialize a Column object? It would seem likely.
func (c *Column) Init() *Column {
	if c == nil {
		c = &Column{}
	}
	c.w = []*Window{}
	c.safe = true
	return c
}

// Add adds a window to the Column.
// If filename is not empty, it is loaded in the window.
func (c *Column) Add(clone *Window, filename string) *Window {
	display, err := drawDev.NewDisplay(nil, *varfontflag, "edward", *winsize)
	if err != nil {
		log.Fatalf("can't open display: %v\n", err)
	}
	if err := display.Attach(draw.Refnone); err != nil {
		panic("failed to attach to window")
	}
	r := display.ScreenImage().R()
	display.ScreenImage().Draw(r, display.White(), nil, image.Point{})

	w := NewWindow()
	w.col = c
	if display != nil {
		display.ScreenImage().Draw(r, w.textcolors[frame.ColBack], nil, image.Point{})
	}
	w.Init(clone, r, display)
	w.tag.col = c
	w.tag.row = c.row
	w.body.col = c
	w.body.row = c.row

	w.keyboardctl = display.InitKeyboard()
	w.mousectl = display.InitMouse()
	w.mouse = &w.mousectl.Mouse

	if filename != "" {
		abspath, _ := filepath.Abs(filename)
		w.SetName(abspath)
		w.body.Load(0, filename, true)
		w.body.file.Clean()
		w.SetTag()
		w.Resize(w.r, false, true)
		w.body.ScrDraw(w.body.fr.GetFrameFillStatus().Nchars)
		w.tag.SetSelect(w.tag.file.Size(), w.tag.file.Size())
	}
	savemouse(w)
	if display != nil {
		display.MoveTo(w.tag.scrollr.Max.Add(image.Pt(3, 3)))
	}
	go mousethread(w)
	go keyboardthread(w)

	c.w = append(c.w, w)
	c.safe = true
	barttext = &w.body
	return w
}

func (c *Column) Close(w *Window, dofree bool) {
	// w is locked
	var i int
	for i = 0; i < len(c.w); i++ {
		if c.w[i] == w {
			goto Found
		}
	}
	acmeerror("can't find window", nil)
Found:
	w.tag.col = nil
	w.body.col = nil
	w.col = nil
	didmouse := restoremouse(w)
	if dofree {
		w.Delete()
		w.Close()
	}
	c.w = append(c.w[:i], c.w[i+1:]...)
	if len(c.w) == 0 {
		return
	}
	up := false
	if i == len(c.w) { // extend last window down
		w = c.w[i-1]
	} else { // extend next window up
		up = true
		w = c.w[i]
	}
	if c.safe && !c.fortest {
		if !didmouse && up {
			w.showdel = true
		}
		if !didmouse && up {
			w.moveToDel()
		}
	}
}

func (c *Column) CloseAll() {
	for _, w := range c.w {
		w.Close()
	}
	clearmouse()
}

func (c *Column) MouseBut() {
}

func (c *Column) Clean() bool {
	clean := true
	for _, w := range c.w {
		clean = w.Clean(true) && clean
	}
	return clean
}
