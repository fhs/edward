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
	display draw.Display
	r       image.Rectangle
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
func (c *Column) Init(r image.Rectangle, dis draw.Display) *Column {
	if c == nil {
		c = &Column{}
	}
	c.display = dis
	c.w = []*Window{}
	if c.display != nil {
		c.display.ScreenImage().Draw(r, c.display.White(), nil, image.Point{})
	}
	c.r = r
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
	if len(c.w) > 0 {
		log.Panicf("can't create more than one window in column")
	}

	r := c.r
	r.Min.Y = 0

	if w == nil {
		w = NewWindow()
		w.col = c
		if c.display != nil {
			c.display.ScreenImage().Draw(r, textcolors[frame.ColBack], nil, image.Point{})
		}
		w.Init(clone, r, c.display)
	} else {
		w.col = c
		w.Resize(r, false, true)
	}
	w.tag.col = c
	w.tag.row = c.row
	w.body.col = c
	w.body.row = c.row
	c.w = append(c.w, w)
	c.safe = true
	savemouse(w)
	if c.display != nil {
		c.display.MoveTo(w.tag.scrollr.Max.Add(image.Pt(3, 3)))
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
	r1 := r
	for i := 0; i < c.nw(); i++ {
		w := c.w[i]
		w.maxlines = 0
		if i == c.nw()-1 {
			r1.Max.Y = r.Max.Y
		} else {
			r1.Max.Y = r1.Min.Y
			if c.r.Dy() != 0 {
				r1.Max.Y += w.r.Dy() * r.Dy() / c.r.Dy()
			}
		}
		r1.Max.Y = max(r1.Max.Y, r1.Min.Y)
		r1.Min.Y = w.Resize(r1, false, i == c.nw()-1)
	}
	c.r = r
}

func (c *Column) Which(p image.Point) *Text {
	if !p.In(c.r) {
		return nil
	}
	for _, w := range c.w {
		if p.In(w.r) {
			if p.In(w.tagtop) || p.In(w.tag.all) {
				return &w.tag
			}
			// exclude partial line at bottom
			if p.X >= w.body.scrollr.Max.X && p.Y >= w.body.fr.Rect().Max.Y {
				return nil
			}
			return &w.body
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
