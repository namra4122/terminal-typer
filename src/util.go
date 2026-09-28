package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gdamore/tcell"
)

var CONFIG_DIRS []string

func init() {
	home, _ := os.LookupEnv("HOME")

	CONFIG_DIRS = []string{
		filepath.Join(home, ".tt"),
		"/etc/tt",
	}
}

type cell struct {
	c     rune
	style tcell.Style
}

func dbgPrintf(scr tcell.Screen, format string, args ...interface{}) {
	for i := 0; i < 80; i++ {
		for j := 0; j < 80; j++ {
			scr.SetContent(i, j, ' ', nil, tcell.StyleDefault)
		}
	}
	drawString(scr, 0, 0, fmt.Sprintf(format, args...), -1, tcell.StyleDefault)
}

func getParagraphs(s string) []string {
	s = strings.Replace(s, "\r", "", -1)
	s = regexp.MustCompile("\n\n+").ReplaceAllString(s, "\n\n")
	return strings.Split(strings.Trim(s, "\n"), "\n\n")
}

func wordWrapBytes(s []byte, n int) {
	sp := 0
	sz := 0

	for i := 0; i < len(s); i++ {
		sz++

		if s[i] == '\n' {
			s[i] = ' '
		}

		if s[i] == ' ' {
			sp = i
		}

		if sz > n {
			if sp != 0 {
				s[sp] = '\n'
			}

			sz = i - sp
		}
	}

}

func wordWrap(s string, n int) string {
	r := []byte(s)
	wordWrapBytes(r, n)
	return string(r)
}

func randomText(n int, words []string) string {
	r := ""

	var last string
	for i := 0; i < n; i++ {
		w := words[rand.IntN(len(words))]
		for last == w {
			w = words[rand.IntN(len(words))]
		}

		r += w
		if i != n-1 {
			r += " "
		}

		last = w
	}

	return strings.Replace(r, "\n", " \n", -1)
}

func stringToCells(s string) []cell {
	a := make([]cell, len(s))
	s = strings.TrimRight(s, "\n ")

	len := 0
	for _, r := range s {
		a[len].c = r
		a[len].style = tcell.StyleDefault
		len++
	}

	return a[:len]
}

func drawString(scr tcell.Screen, x, y int, s string, cursorIdx int, style tcell.Style) {
	sx := x

	for i, c := range s {
		if c == '\n' {
			y++
			x = sx
		} else {
			scr.SetContent(x, y, c, nil, style)
			if i == cursorIdx {
				scr.ShowCursor(x, y)
			}

			x++
		}
	}

	if cursorIdx == len(s) {
		scr.ShowCursor(x, y)
	}
}

func drawStringAtCenter(scr tcell.Screen, s string, style tcell.Style) {
	nc, nr := calcStringDimensions(s)
	sw, sh := scr.Size()

	x := (sw - nc) / 2
	y := (sh - nr) / 2

	drawString(scr, x, y, s, -1, style)
}

func calcStringDimensions(s string) (nc, nr int) {
	if s == "" {
		return 0, 0
	}

	c := 0

	for _, x := range s {
		if x == '\n' {
			nr++
			if c > nc {
				nc = c
			}
			c = 0
		} else {
			c++
		}
	}

	nr++
	if c > nc {
		nc = c
	}

	return
}

func newTcellColor(s string) (tcell.Color, error) {
	if len(s) != 7 || s[0] != '#' {
		return 0, fmt.Errorf("%s is not a valid hex color", s)
	}

	tonum := func(c byte) int32 {
		if c > '9' {
			if c >= 'a' {
				return (int32)(c - 'a' + 10)
			} else {
				return (int32)(c - 'A' + 10)
			}
		} else {
			return (int32)(c - '0')
		}
	}

	r := tonum(s[1])<<4 | tonum(s[2])
	g := tonum(s[3])<<4 | tonum(s[4])
	b := tonum(s[5])<<4 | tonum(s[6])

	return tcell.NewRGBColor(r, g, b), nil
}

type ResourceOrigin struct {
	Kind     string
	PackID   string
	Revision string
	Path     string
	Embedded bool
}

func resourceKind(typ string, embedded bool) string {
	kind := strings.TrimSuffix(typ, "s")
	if embedded {
		return "embedded-" + kind
	}
	return "private-" + kind
}

func ResolveResource(typ, name string) ([]byte, ResourceOrigin, error) {
	if name == "-" {
		b, err := io.ReadAll(os.Stdin)
		return b, ResourceOrigin{Kind: resourceKind(typ, false), Path: "-"}, err
	}

	if b, err := os.ReadFile(name); err == nil {
		return b, ResourceOrigin{Kind: resourceKind(typ, false), Path: name}, nil
	}

	for _, d := range CONFIG_DIRS {
		path := filepath.Join(d, typ, name)
		if b, err := os.ReadFile(path); err == nil {
			return b, ResourceOrigin{Kind: resourceKind(typ, false), Path: path}, nil
		}
	}

	b := readPackedFile(filepath.Join(typ, name))
	if b == nil {
		return nil, ResourceOrigin{}, fmt.Errorf("%s resource %q not found", typ, name)
	}
	sum := sha256.Sum256(b)
	return b, ResourceOrigin{
		Kind: resourceKind(typ, true), PackID: name,
		Revision: hex.EncodeToString(sum[:]), Embedded: true,
	}, nil
}

func readResource(typ, name string) []byte {
	b, _, err := ResolveResource(typ, name)
	if err != nil {
		if name == "-" {
			panic(err)
		}
		return nil
	}
	return b
}
