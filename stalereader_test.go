package pdffont_test

import (
	"testing"
	"time"

	"github.com/go-pdfkit/pdffont"
	"github.com/go-pdfkit/reader"
)

// hugeObjectNumber is a whole PDF in 219 bytes. It has no trailer and no
// startxref, so it can only be read by repairing it, and the last object it
// declares is numbered 2 147 483 647.
//
// reader v0.4.0 answered "which objects call themselves a catalogue?" by
// counting from zero to the largest object number the file mentioned, one map
// lookup each. For this file that is two thousand million lookups for four
// objects: twenty-one seconds, and not one byte allocated, which is why no
// memory limit anywhere caught it.
const hugeObjectNumber = "%PDF-1.7\n" +
	"1 0 obj <</Type /Catalog /Pages 2 0 R>>\nendobj\n" +
	"2 0 obj <</Type /Pages /Kids [3 0 R] /Count 1>>\nendobj\n" +
	"3 0 obj <</Type /Page /Parent 2 0 R /MediaBox [0 0 10 10]>>\nendobj\n\n" +
	"2147483647 0 obj <</Root 1 0 R>>\nendobj\n"

// TestATinyFileWithAHugeObjectNumber guards the version of the reader this
// package is built against, not this package's own code.
//
// The defect was fixed in reader v0.4.1, and merging is not shipping: a
// consumer still asking for v0.4.0 still hands its callers a file that takes
// twenty-one seconds to open. The budget is two seconds because the answer is
// either a fraction of a millisecond or twenty-one seconds, and nothing in
// between; there is no threshold here to tune.
func TestATinyFileWithAHugeObjectNumber(t *testing.T) {
	b := []byte(hugeObjectNumber)
	start := time.Now()
	d, err := reader.Open(b)
	opened := time.Since(start)
	if err != nil {
		t.Fatalf("opening %d bytes: %v", len(b), err)
	}
	if opened > 2*time.Second {
		t.Fatalf("opening %d bytes took %s: the reader this is built against walks every object "+
			"number up to the largest one named, so it is older than v0.4.1", len(b), opened)
	}
	// And through this package's own entry point, since that is what callers
	// reach it by: a font dictionary is read out of a document.
	page, err := d.Page(1)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	resources, _ := d.GetDict(page, "Resources")
	fonts, _ := d.GetDict(resources, "Font")
	start = time.Now()
	for k := range fonts {
		if fd, ok := d.GetDict(fonts, k); ok {
			pdffont.Read(d, fd)
		}
	}
	if read := time.Since(start); read > 2*time.Second {
		t.Fatalf("reading the fonts of a %d-byte file took %s", len(b), read)
	}
}
