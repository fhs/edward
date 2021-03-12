package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/fhs/edward/internal/dumpfile"
)

const RowTag = "Newcol Kill Putall Dump Exit "

type Row struct {
	lk  sync.Mutex
	col Column
}

func (row *Row) Init(dump *dumpfile.Content, loadfile string) *Row {
	if row == nil {
		row = &Row{}
	}
	// we only support one column
	c := &row.col
	c.Init()
	c.row = row
	clearmouse()

	if loadfile == "" || row.Load(dump, loadfile) != nil {
		readArgFiles(flag.Args())
	}
	return row
}

func (row *Row) Type(w *Window, r rune, p image.Point) *Text {
	if r == 0 {
		r = utf8.RuneError
	}

	clearmouse()
	row.lk.Lock()
	var t *Text
	if *barflag {
		t = barttext
	} else {
		t = w.Which(p)
	}
	if t != nil && !(t.what == Tag && p.In(t.scrollr)) {
		w := t.w
		if w == nil {
			// Texts in column tags or the very top.
			t.Type(r)
		} else {
			w.Lock('K')
			w.Type(t, r)
			// Expand tag if necessary
			if t.what == Tag {
				t.w.tagsafe = false
				if r == '\n' {
					t.w.tagexpand = true
				}
				w.Resize(w.r, true, true)
			}
			w.Unlock()
		}
	}
	row.lk.Unlock()
	return t
}

func (row *Row) Clean() bool {
	return row.col.Clean()
}

func (r *Row) Dump(file string) error {
	if file == "" {
		f, err := defaultDumpFile()
		if err != nil {
			return warnError(nil, "can't find file for dump: %v", err)
		}
		file = f
	}
	dump, err := r.dump()
	if err != nil {
		return err
	}

	if err := dump.Save(file); err != nil {
		return warnError(nil, "dumping to %v failed: %v", file, err)
	}
	return nil
}

func (r *Row) dump() (*dumpfile.Content, error) {
	rowTag := string(RowTag)
	// Remove commands at the beginning of row tag.
	if i := strings.Index(rowTag, RowTag); i > 1 {
		rowTag = rowTag[i:]
	}
	dump := &dumpfile.Content{
		CurrentDir: wdir,
		VarFont:    *varfontflag,
		FixedFont:  *fixedfontflag,
		RowTag: dumpfile.Text{
			Buffer: rowTag,
			Q0:     0,
			Q1:     0,
		},
		Columns: []dumpfile.Column{
			{
				Position: 0,
				Tag: dumpfile.Text{
					Buffer: string(Lheader),
					Q0:     0,
					Q1:     0,
				},
			},
		},
		Windows: nil,
	}

	dumpid := make(map[*File]int)
	for _, w := range r.col.w {
		if w.nopen[QWevent] != 0 {
			// Mark zeroxes of external windows specially.
			dumpid[w.body.file] = -1
		}
	}

	for _, w := range r.col.w {
		// Do we need to Commit on the other tags?
		w.Commit(&w.tag)
		t := &w.body

		// External windows can't be recreated so skip them.
		if w.nopen[QWevent] > 0 {
			if w.dumpstr == "" {
				continue
			}
		}

		// zeroxes of external windows are tossed
		if dumpid[t.file] < 0 && w.nopen[QWevent] == 0 {
			continue
		}

		// We always include the font name.
		fontname := t.font

		dump.Windows = append(dump.Windows, &dumpfile.Window{
			Column: 0,
			Body: dumpfile.Text{
				Buffer: "", // filled in later if Unsaved
				Q0:     w.body.q0,
				Q1:     w.body.q1,
			},
			Position: 0,
			Font:     fontname,
		})
		dw := dump.Windows[len(dump.Windows)-1]

		switch {
		case dumpid[t.file] > 0:
			dw.Type = dumpfile.Zerox

		case w.dumpstr != "":
			dw.Type = dumpfile.Exec
			dw.ExecDir = w.dumpdir
			dw.ExecCommand = w.dumpstr

		case !w.body.file.Dirty() && access(t.file.name) || w.body.file.IsDir():
			dumpid[t.file] = w.id
			dw.Type = dumpfile.Saved

		default:
			dumpid[t.file] = w.id
			// TODO(rjk): Conceivably this is a bit of a layering violation?
			dw.Type = dumpfile.Unsaved
			dw.Body.Buffer = string(t.file.b)
		}
		dw.Tag = dumpfile.Text{
			Buffer: string(w.tag.file.b),
			Q0:     w.tag.q0,
			Q1:     w.tag.q1,
		}
	}
	return dump, nil
}

// loadhelper breaks out common load file parsing functionality for selected row
// types.
func (row *Row) loadhelper(win *dumpfile.Window) error {
	// Column for this window.
	c := &row.col
	y := -1

	subl := strings.SplitN(win.Tag.Buffer, " ", 2)
	if len(subl) != 2 {
		return fmt.Errorf("bad window tag in dump file %q", win.Tag)
	}

	var w *Window
	if win.Type != dumpfile.Zerox {
		w = c.Add(nil, nil, y)
	} else {
		w = c.Add(nil, lookfile(subl[0]), y)
	}
	if w == nil {
		// Why is this not an error?
		return nil
	}

	if win.Type != dumpfile.Zerox {
		w.SetName(subl[0])
	}

	// TODO(rjk): I feel that the code for managing tags could be extracted and unified.
	// Maybe later. Window.setTag1 would seem fixable.
	afterbar := strings.SplitN(subl[1], "|", 2)
	if len(afterbar) != 2 {
		return fmt.Errorf("bad window tag in dump file %q", win.Tag)
	}
	w.ClearTag()
	w.tag.Insert(len(w.tag.file.b), []rune(afterbar[1]), true)
	w.tag.Show(win.Tag.Q0, win.Tag.Q1, true)

	if win.Type == dumpfile.Unsaved {
		w.body.LoadReader(0, subl[0], strings.NewReader(win.Body.Buffer), true)
		w.body.file.Modded()

		// This shows an example where an observer would be useful?
		w.SetTag()
	} else if win.Type != dumpfile.Zerox && len(subl[0]) > 0 && subl[0][0] != '+' && subl[0][0] != '-' {
		// Implementation of the Get command: open the file.
		get(&w.body, nil, nil, false, false, "")
	}

	if win.Font != "" {
		fontx(&w.body, nil, nil, false, false, win.Font)
	}

	q0 := win.Body.Q0
	q1 := win.Body.Q1
	if q0 > len(w.body.file.b) || q1 > len(w.body.file.b) || q0 > q1 {
		q0 = 0
		q1 = 0
	}
	// Update the selection on the Text.
	w.body.Show(q0, q1, true)
	ffs := w.body.fr.GetFrameFillStatus()
	w.maxlines = min(ffs.Nlines, max(w.maxlines, ffs.Nlines))

	// TODO(rjk): Conceivably this should be a zerox xfidlog when reconstituting a zerox?
	xfidlog(w, "new")
	return nil
}

// Load restores Edwood's state stored in dump. If dump is nil, it is parsed from file.
// If initing is true, Row will be initialized.
func (row *Row) Load(dump *dumpfile.Content, file string) error {
	if dump == nil {
		if file == "" {
			f, err := defaultDumpFile()
			if err != nil {
				return warnError(nil, "can't find file for load: %v", err)
			}
			file = f
		}
		d, err := dumpfile.Load(file)
		if err != nil {
			return warnError(nil, "can't load dump file: %v", err)
		}
		dump = d
	}
	err := row.loadimpl(dump)
	if err != nil {
		return warnError(nil, "can't load row: %v", err)
	}
	return err
}

// TODO(rjk): split this apart into smaller functions and files.
func (row *Row) loadimpl(dump *dumpfile.Content) error {
	// log.Println("Load start", file, initing)
	// defer log.Println("Load ended")

	// Current directory.
	if err := os.Chdir(dump.CurrentDir); err != nil {
		return err
	}
	wdir = dump.CurrentDir

	// variable width font
	*varfontflag = dump.VarFont

	// fixed width font
	*fixedfontflag = dump.FixedFont

	// Column widths
	if len(dump.Columns) > 10 {
		return fmt.Errorf("Load: bad number of columns %d", len(dump.Columns))
	}

	// TODO(rjk): put column width parsing in a separate function.
	for _, col := range dump.Columns {
		percent := col.Position
		if percent < 0 || percent >= 100 {
			return fmt.Errorf("Load: column width %f is invalid", percent)
		}
	}

	// Load the windows.
	for _, win := range dump.Windows {
		switch win.Type {
		case dumpfile.Exec: // command block
			dirline := win.ExecDir
			if dirline == "" {
				dirline = home
			}
			// log.Println("cmdline", cmdline, "dirline", dirline)
			run(nil, win.ExecCommand, dirline, true, "", "", false)

		case dumpfile.Saved, dumpfile.Unsaved, dumpfile.Zerox:
			if err := row.loadhelper(win); err != nil {
				return err
			}

		default:
			return fmt.Errorf("unknown dump file window type %v", win.Type)
		}
	}
	return nil
}

func (r *Row) AllWindows(f func(*Window)) {
	for _, w := range r.col.w {
		f(w)
	}
}

func (r *Row) LookupWin(id int) *Window {
	for _, w := range r.col.w {
		if w.id == id {
			return w
		}
	}
	return nil
}

func defaultDumpFile() (string, error) {
	if home == "" {
		return "", fmt.Errorf("can't find home directory")
	}
	// Lower risk of simultaneous use of edwood and acme.
	return filepath.Join(home, "edwood.dump"), nil
}
