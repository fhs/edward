package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/fhs/edward/internal/runes"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func minu(a, b uint) uint {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func acmeerror(s string, err error) {
	log.Panicf("acme: %s: %v\n", s, err)
}

func bytetorune(s []byte) []rune {
	r, _, _ := cvttorunes(s, len(s))
	return r
}

// TODO(flux) The "correct" answer here is return unicode.IsNumber(c) || unicode.IsLetter(c)
func isalnum(c rune) bool {
	// Hard to get absolutely right.  Use what we know about ASCII
	// and assume anything above the Latin control characters is
	// potentially an alphanumeric.
	if c <= ' ' {
		return false
	}
	if 0x7F <= c && c <= 0xA0 {
		return false
	}
	if strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^`{|}~", c) {
		return false
	}
	return true
}

// Cvttorunes decodes runes r from p. It's guaranteed that first n
// bytes of p will be interpreted without worrying about partial runes.
// This may mean reading up to UTFMax-1 more bytes than n; the caller
// must ensure p is large enough. Partial runes and invalid encodings
// are converted to RuneError. Nb (always >= n) is the number of bytes
// interpreted.
//
// If any U+0000 rune is present in r, they are elided and nulls is set
// to true.
func cvttorunes(p []byte, n int) (r []rune, nb int, nulls bool) {
	for nb < n {
		var w int
		var ru rune
		if p[nb] < utf8.RuneSelf {
			w = 1
			ru = rune(p[nb])
		} else {
			ru, w = utf8.DecodeRune(p[nb:])
		}
		if ru != 0 {
			r = append(r, ru)
		} else {
			nulls = true
		}
		nb += w
	}
	return
}

func errorwin1Name(dir string) string {
	return filepath.Join(dir, "+Errors")
}

func errorwin1(dir string, incl []string) *Window {
	r := errorwin1Name(dir)
	w := lookfile(r)
	if w == nil {
		w = row.col.Add(nil, -1)
		defer w.HandleInput()
		w.filemenu = false
		w.SetName(r)
		xfidlog(w, "new")
	}
	for _, in := range incl {
		w.AddIncl(in)
	}
	w.autoindent = *globalAutoIndent
	return w
}

// make new window, if necessary; return with it locked
func errorwin(md *MntDir, owner int) *Window {
	var w *Window

	for {
		if md == nil {
			w = errorwin1("", nil)
		} else {
			w = errorwin1(md.dir, md.incl)
		}
		w.Lock(owner)
		if w.col != nil {
			break
		}
		// window was deleted too fast
		w.Unlock()
	}
	return w
}

// Incoming window should be locked.
// It will be unlocked and returned window
// will be locked in its place.
func errorwinforwin(w *Window) *Window {
	var (
		owner int
		incl  []string
		t     *Text
	)

	t = &w.body
	dir := t.DirName("")
	incl = append(incl, w.incl...)
	owner = w.owner
	w.Unlock()
	for {
		w = errorwin1(dir, incl)
		w.Lock(owner)
		if w.col != nil {
			break
		}
		// window deleted too fast
		w.Unlock()
	}
	return w
}

func makenewwindow(t *Text) *Window {
	return row.col.Add(nil, -1)
}

type Warning struct {
	md  *MntDir
	buf Buffer
}

var warnings = []*Warning{}
var warningsMu sync.Mutex

func flushwarnings() {
	var (
		w                *Window
		t                *Text
		owner, nr, q0, n int
	)
	for _, warn := range warnings {
		w = errorwin(warn.md, 'E')
		t = &w.body
		owner = w.owner
		if owner == 0 {
			w.owner = 'E'
		}
		w.Commit(t) // marks the backing text as dirty

		// Most commands don't generate much output. For instance,
		// Edit ,>cat goes through /dev/cons and is already in blocks
		// because of the i/o system, but a few can.  Edit ,p will
		// put the entire result into a single hunk.  So it's worth doing
		// this in blocks (and putting the text in a buffer in the first
		// place), to avoid a big memory footprint.
		q0 = t.Nc()
		r := make([]rune, RBUFSIZE)
		// TODO(rjk): Figure out why Warning doesn't use a File.
		for n = 0; n < warn.buf.nc(); n += nr {
			nr = warn.buf.nc() - n
			if nr > RBUFSIZE {
				nr = RBUFSIZE
			}
			warn.buf.Read(n, r[:nr])
			_, nr = t.BsInsert(t.Nc(), r[:nr], true)
		}
		t.Show(q0, t.Nc(), true)
		t.w.SetTag()
		t.ScrDraw(t.fr.GetFrameFillStatus().Nchars)
		w.owner = owner
		t.file.TreatAsClean()
		w.Unlock()
		// warn.buf.Close()
		if warn.md != nil {
			mnt.DecRef(warn.md) // IncRef in addwarningtext
		}
	}
	warnings = warnings[0:0]
}

func warning(md *MntDir, s string, args ...interface{}) {
	r := []rune(fmt.Sprintf(s, args...))
	addwarningtext(md, r)
}

func warnError(md *MntDir, s string, args ...interface{}) error {
	err := fmt.Errorf(s, args...)
	addwarningtext(md, []rune(err.Error()+"\n"))
	return err
}

func addwarningtext(md *MntDir, r []rune) {
	warningsMu.Lock()
	defer warningsMu.Unlock()

	for _, warn := range warnings {
		if warn.md == md {
			warn.buf.Insert(warn.buf.nc(), r)
			return
		}
	}
	warn := Warning{}
	warn.md = md
	if md != nil {
		mnt.IncRef(md) // DecRef in flushwarnings
	}
	warn.buf.Insert(0, r)
	warnings = append(warnings, &warn)
	select {
	case cwarn <- 0:
	default:
	}
}

const quoteChar = '\''

func needsQuote(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == quoteChar || c <= ' ' { // quote, blanks, or control characters
			return true
		}
	}
	return false
}

// Quote adds single quotes to s in the style of rc(1) if they are needed.
// The behaviour should be identical to Plan 9's quote(3).
func quote(s string) string {
	if s == "" {
		return "''"
	}
	if !needsQuote(s) {
		return s
	}
	var b strings.Builder
	b.Grow(10 + len(s)) // Enough room for few quotes
	b.WriteByte(quoteChar)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == quoteChar {
			b.WriteByte(quoteChar)
		}
		b.WriteByte(c)
	}
	b.WriteByte(quoteChar)
	return b.String()
}

func skipbl(r []rune) []rune {
	return runes.TrimLeft(r, " \t\n")
}
