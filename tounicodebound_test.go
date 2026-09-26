package pdffont_test

import (
	"bytes"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/go-pdfkit/pdffont"
)

// manyRanges builds a CMap holding n bfrange blocks, each naming a distinct
// block of 65 536 codes. That is about twenty bytes of input for sixty-five
// thousand entries of output, which is the whole of the defect: the width of
// one run was bounded and the number of runs was not, so the size of the
// answer had nothing to do with the size of the question.
func manyRanges(n int) []byte {
	var b bytes.Buffer
	b.WriteString("/CIDInit /ProcSet findresource begin\nbegincmap\n")
	for i := 0; i < n; i++ {
		b.WriteString("1 beginbfrange\n")
		fmt.Fprintf(&b, "<%04x0000><%04xffff><0041>\n", i, i)
		b.WriteString("endbfrange\n")
	}
	b.WriteString("endcmap\n")
	return b.Bytes()
}

// TestToUnicodeIsBoundedByItsOwnSize is the regression. Before the bound,
// 10 655 bytes of CMap produced 13 107 200 entries, a gigabyte of memory and
// ten seconds; every font on every page is read this way, so that was a
// gigabyte per font.
//
// What is asserted is that the cost stops growing: sixteen times the input
// must not buy sixteen times the entries.
func TestToUnicodeIsBoundedByItsOwnSize(t *testing.T) {
	const limit = 1 << 18
	small := pdffont.ReadToUnicode(manyRanges(10))
	large := pdffont.ReadToUnicode(manyRanges(2000))
	if len(small) > limit {
		t.Fatalf("585 bytes named %d codes, over the %d bound", len(small), limit)
	}
	if len(large) > limit {
		t.Fatalf("106 055 bytes named %d codes, over the %d bound", len(large), limit)
	}
	if len(large) != len(small) {
		t.Fatalf("two hundred times the input gave %d codes against %d: the bound is not holding",
			len(large), len(small))
	}
}

// TestToUnicodeCostIsBounded checks the other half: time and memory, measured
// rather than assumed. The generous limits are there so the test says
// something on a loaded machine and still fails loudly on an unbounded one,
// which used to spend ten seconds and a gigabyte here.
func TestToUnicodeCostIsBounded(t *testing.T) {
	data := manyRanges(2000)
	var m0, m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m0)
	start := time.Now()
	got := pdffont.ReadToUnicode(data)
	elapsed := time.Since(start)
	runtime.ReadMemStats(&m1)
	allocated := float64(m1.TotalAlloc-m0.TotalAlloc) / (1 << 20)

	if elapsed > 5*time.Second {
		t.Errorf("%d bytes of CMap took %s", len(data), elapsed)
	}
	if allocated > 250 {
		t.Errorf("%d bytes of CMap allocated %.0f MB", len(data), allocated)
	}
	if len(got) == 0 {
		t.Fatal("the map came back empty; the bound should cut it off, not throw it away")
	}
}

// TestToUnicodeStillReadsWhatItShould checks the bound did not break the maps
// anybody actually has. The largest of 5 338 /ToUnicode maps taken out of real
// documents names 65 536 codes — one whole two-byte code space — so a map of
// that size has to come back whole.
func TestToUnicodeStillReadsWhatItShould(t *testing.T) {
	full := []byte("/CIDInit /ProcSet findresource begin\nbegincmap\n" +
		"1 beginbfrange\n<0000><ffff><0041>\nendbfrange\nendcmap\n")
	m := pdffont.ReadToUnicode(full)
	if len(m) != 65536 {
		t.Fatalf("a full two-byte code space came back with %d codes, want 65536", len(m))
	}
	if m[0] != "A" {
		t.Errorf("code 0 stands for %q, want %q", m[0], "A")
	}
	if m[1] != "B" {
		t.Errorf("code 1 stands for %q, want %q — a range counts on from its first character", m[1], "B")
	}

	// And the ordinary small map, which is what nearly every document has:
	// the median of those 5 338 names thirteen codes.
	small := pdffont.ReadToUnicode([]byte("begincmap\n2 beginbfchar\n<0041><0061>\n<0042><0062>\nendbfchar\nendcmap\n"))
	if len(small) != 2 {
		t.Fatalf("a two-entry bfchar block gave %d codes", len(small))
	}
	if small[0x41] != "a" || small[0x42] != "b" {
		t.Errorf("bfchar gave %q and %q", small[0x41], small[0x42])
	}
}

// TestToUnicodeBoundHoldsWithinOneBlock covers the other shape the same
// document can take: not many blocks of one range each, but one block naming
// many. The bound has to hold inside a block as well as between them, and a
// block that follows a map already full must stop rather than start.
func TestToUnicodeBoundHoldsWithinOneBlock(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("/CIDInit /ProcSet findresource begin\nbegincmap\n")
	b.WriteString("400 beginbfrange\n")
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&b, "<%04x0000><%04xffff><0041>\n", i, i)
	}
	b.WriteString("endbfrange\n")
	// A second block, entered with the map already full: it must add nothing
	// rather than run to the end of its own list.
	b.WriteString("2 beginbfchar\n<0041><0061>\n<0042><0062>\nendbfchar\n")
	b.WriteString("1 beginbfrange\n<ffff0000><ffffffff><0041>\nendbfrange\n")
	b.WriteString("endcmap\n")

	start := time.Now()
	m := pdffont.ReadToUnicode(b.Bytes())
	elapsed := time.Since(start)
	if len(m) > 1<<18 {
		t.Fatalf("one block of 400 ranges named %d codes, over the bound", len(m))
	}
	if elapsed > 5*time.Second {
		t.Fatalf("one block of 400 ranges took %s", elapsed)
	}
	// The blocks after the map filled must have been refused outright, so the
	// codes they name are not there.
	// The key is computed the way the package computes it, not written out as
	// a constant. A code is assembled into an int a byte at a time, so a
	// four-byte code lands on a DIFFERENT NUMBER depending on how wide an int
	// is: 4 294 901 760 where int is 64 bits, minus 65 536 where it is 32. The
	// constant did not merely read the wrong key on a 32-bit build -- it would
	// not compile there at all, which is how the 386 and arm lanes found it.
	key := 0
	for _, c := range []byte{0xff, 0xff, 0x00, 0x00} {
		key = key<<8 | int(c)
	}
	if _, ok := m[key]; ok {
		t.Error("a range block entered with a full map added to it anyway")
	}
}

// TestToUnicodeBFCharBlockAfterFull covers the char block's own guard with
// nothing else in the way: a bfchar block whose pairs begin after the bound
// has already been reached.
func TestToUnicodeBFCharBlockAfterFull(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("begincmap\n400 beginbfrange\n")
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&b, "<%04x0000><%04xffff><0041>\n", i, i)
	}
	// Codes far outside every range above, so that finding one in the answer
	// can only mean the char block was read.
	b.WriteString("endbfrange\n4 beginbfchar\n")
	for i := 0; i < 4; i++ {
		fmt.Fprintf(&b, "<7fff00%02x><0061>\n", i)
	}
	b.WriteString("endbfchar\nendcmap\n")
	m := pdffont.ReadToUnicode(b.Bytes())
	if _, ok := m[0x7fff0000]; ok {
		t.Error("a char block entered with a full map added to it anyway")
	}
	if len(m) > 1<<18 {
		t.Fatalf("named %d codes, over the bound", len(m))
	}
}
