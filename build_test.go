package pdffont

import (
	"testing"

	"github.com/go-pdfkit/reader"
)

// fontDoc builds a document holding one font dictionary and reads it back the
// way a page would.
func fontDoc(t *testing.T, build func(w *reader.Writer) reader.Dict) *Font {
	t.Helper()
	d, dict := fontIn(t, build)
	return Read(d, dict)
}

// fontIn builds the document and gives back both it and the font dictionary,
// for a test that needs the document too.
func fontIn(t *testing.T, build func(w *reader.Writer) reader.Dict) (*reader.Document, reader.Dict) {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	font := build(w)
	page := w.Add(reader.Dict{"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox":  reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(10), reader.Integer(10)},
		"Resources": reader.Dict{"Font": reader.Dict{"F1": w.Add(font)}}})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1)})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	d, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(1)
	if err != nil {
		t.Fatal(err)
	}
	res, _ := d.GetDict(p, "Resources")
	fonts, _ := d.GetDict(res, "Font")
	dict, ok := d.GetDict(fonts, "F1")
	if !ok {
		t.Fatal("the font is not there")
	}
	return d, dict
}

// widthArray is a run of widths, in the thousandths a font is measured in.
func widthArray(vs ...int) reader.Array {
	out := make(reader.Array, 0, len(vs))
	for _, v := range vs {
		out = append(out, reader.Integer(int64(v)))
	}
	return out
}
