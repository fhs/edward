package main

import (
	"image"
	"log"

	"github.com/rjkroege/edwood/internal/draw"
	"github.com/rjkroege/edwood/internal/frame"
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

// findWindowContainingY finds the window containing vertical offset y
// and returns the Window and its index.
// TODO(rjk): It's almost certain that we repeat this code somewhere else.
// possibly multiple times.
// TODO(rjk): Get rid of the index requirement?
func (c *Column) findWindowContainingY(y int) (i int, v *Window) {
	for i, v = range c.w {
		if y < v.r.Max.Y {
			return i, v
		}
	}
	return len(c.w), v
}

// Add adds a window to the Column.
// TODO(rjk): what are the args?
func (c *Column) Add(w, clone *Window, y int) *Window {
	display, err := drawDev.NewDisplay(nil, *varfontflag, "edwood", *winsize)
	if err != nil {
		log.Fatalf("can't open display: %v\n", err)
	}
	if err := display.Attach(draw.Refnone); err != nil {
		panic("failed to attach to window")
	}
	r := display.ScreenImage().R()
	display.ScreenImage().Draw(r, display.White(), nil, image.Point{})

	if w == nil {
		w = NewWindow()
		iconinit(display, &w.iconImages)
		w.col = c
		if display != nil {
			display.ScreenImage().Draw(r, textcolors[frame.ColBack], nil, image.Point{})
		}
		w.Init(clone, r, display)
	} else {
		w.col = c
		w.Resize(r, false, true)
	}
	w.tag.col = c
	w.tag.row = c.row
	w.body.col = c
	w.body.row = c.row

	w.keyboardctl = display.InitKeyboard()
	w.mousectl = display.InitMouse()
	mouse = &w.mousectl.Mouse
	go mousethread(w)
	go keyboardthread(w)

	c.w = append(c.w, w)
	c.safe = true
	savemouse(w)
	if display != nil {
		display.MoveTo(w.tag.scrollr.Max.Add(image.Pt(3, 3)))
	}
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
		log.Panicf("closing window is not implemented")
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
	if c == activecol {
		activecol = nil
	}
	for _, w := range c.w {
		w.Close()
	}
	clearmouse()
}

func (c *Column) MouseBut() {
}

func (c *Column) Resize(r image.Rectangle) {
	clearmouse()
	for i := 0; i < c.nw(); i++ {
		w := c.w[i]
		w.maxlines = 0
		w.Resize(r, false, i == c.nw()-1)
	}
}

func (c *Column) Which(p image.Point) *Text {
	for _, w := range c.w {
		if p.In(w.r) {
			return w.Which(p)
		}
	}
	return nil
}

func (c *Column) Clean() bool {
	clean := true
	for _, w := range c.w {
		clean = w.Clean(true) && clean
	}
	return clean
}
