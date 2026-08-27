package pdffont_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-pdfkit/pdffont"
)

// seedDir can be pointed at a directory of /ToUnicode maps taken out of real
// documents — CMAP_SEEDS — which is a far better starting population than
// anything a generator would invent. Without it the built-in seeds and the
// crashers under testdata still run.
var seedDir = os.Getenv("CMAP_SEEDS")

func addCMapSeeds(f *testing.F, max, cap int) {
	f.Add([]byte("begincmap\n1 beginbfchar\n<0041><0061>\nendbfchar\nendcmap\n"))
	f.Add([]byte("begincmap\n1 beginbfrange\n<0000><00ff><0041>\nendbfrange\nendcmap\n"))
	f.Add([]byte("begincmap\n1 beginbfrange\n<0000><0002>[<0041><0042><0043>]\nendbfrange\nendcmap\n"))
	if seedDir == "" {
		return
	}
	ents, err := os.ReadDir(seedDir)
	if err != nil {
		return
	}
	n := 0
	for _, e := range ents {
		info, err := e.Info()
		if err != nil || info.Size() > int64(cap) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(seedDir, e.Name()))
		if err != nil {
			continue
		}
		f.Add(b)
		if n++; n >= max {
			return
		}
	}
}

// FuzzReadToUnicode drives the CMap parser, which reads a little program the
// document supplies whose ranges are numbers the document chose.
//
// The budget is the point of the target as much as the panic is: a bfrange
// names a run of codes in about twenty bytes, so the answer's size need not
// have anything to do with the question's, and a cost that grows without the
// input growing raises nothing on its own.
func FuzzReadToUnicode(f *testing.F) {
	addCMapSeeds(f, 800, 32*1024)
	f.Fuzz(func(t *testing.T, b []byte) {
		start := time.Now()
		m := pdffont.ReadToUnicode(b)
		if d := time.Since(start); d > 2*time.Second {
			t.Fatalf("%d bytes of CMap took %s and named %d codes", len(b), d, len(m))
		}
		if len(m) > 1<<18 {
			t.Fatalf("%d bytes of CMap named %d codes", len(b), len(m))
		}
	})
}

// FuzzRuneOfGlyphName drives the other place a document's own text becomes a
// lookup: the glyph names a font gives its characters.
func FuzzRuneOfGlyphName(f *testing.F) {
	for _, s := range []string{
		"A", "space", "uni0041", "u1F600", "g123", "cid42", "afii10017",
		"uni", "uniZZZZ", "u", "", "a.sc", "f_f_i", "uni00410042",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		start := time.Now()
		pdffont.RuneOfGlyphName(s)
		pdffont.TrimText(s)
		if d := time.Since(start); d > time.Second {
			t.Fatalf("a %d-byte glyph name took %s", len(s), d)
		}
	})
}
