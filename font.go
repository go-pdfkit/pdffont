// Package pdffont is the PDF side of a font: not the outlines, which are the
// font program's business, but everything the document itself says about how a
// string of bytes is to be read. Which codes a string holds, how wide each one
// is, what each is called, and what text each stands for.
//
// It is what a renderer needs before it can ask a font program for a glyph,
// and all an extractor needs to read a page back as words. Both would
// otherwise have to say the same things twice.
package pdffont

import (
	"github.com/go-pdfkit/reader"
)

// A Kind says how a font's codes are read.
type Kind int

const (
	// Simple fonts take one byte a code and are addressed by glyph name.
	Simple Kind = iota
	// Composite fonts take a character identifier, usually two bytes wide.
	Composite
	// Type3 fonts draw each glyph by running a little content stream.
	Type3
)

// A Font is a font as the document describes it.
type Font struct {
	kind       Kind
	dict       reader.Dict
	descriptor reader.Dict
	doc        *reader.Document

	names map[int]string // what each code is called, for a simple font
	// chosen says which of those names the document chose itself, in a
	// Differences array, rather than inheriting from a base encoding.
	chosen   map[int]bool
	widths   map[int]float64 // in text space: a thousandth of the point size
	defaultW float64
	toUni    map[int]string
	symbolic bool

	// fallback is a last resort for codes the document itself says nothing
	// about: what the embedded font program calls them. A caller that can
	// read the program installs one.
	fallback func(code int) (string, bool)

	// cidToGID maps a character identifier to a glyph number, for a composite
	// font whose descendant carries a map rather than using them as they are.
	cidToGID []byte

	// The pieces only a Type 3 font has.
	fontMatrix [6]float64
	charProcs  reader.Dict
	t3Res      reader.Dict
}

// Read takes what a font dictionary says. It never fails: a dictionary that
// says nothing useful gives a font that reads its codes one byte at a time and
// advances by half an em, which is what a reader has to do with one anyway.
func Read(d *reader.Document, dict reader.Dict) *Font {
	f := &Font{
		doc: d, dict: dict, widths: map[int]float64{}, defaultW: 0.5,
		fontMatrix: [6]float64{0.001, 0, 0, 0.001, 0, 0},
	}
	switch sub, _ := reader.ToName(resolve(d, dict.Get("Subtype"))); sub {
	case "Type0":
		f.kind = Composite
		f.readComposite()
	case "Type3":
		f.kind = Type3
		f.readType3()
	default:
		f.kind = Simple
		f.readSimple()
	}
	f.toUni = f.readToUnicode()
	return f
}

// Kind says how this font's codes are read.
func (f *Font) Kind() Kind { return f.kind }

// Dict is the font dictionary itself, for whatever a caller needs that this
// does not say.
func (f *Font) Dict() reader.Dict { return f.dict }

// Descriptor is the font descriptor, which is where an embedded program lives.
func (f *Font) Descriptor() reader.Dict { return f.descriptor }

// Symbolic reports a font that says it is addressed through its own character
// map rather than through an encoding of names.
func (f *Font) Symbolic() bool { return f.symbolic }

// FontMatrix is how a Type 3 font's glyph space relates to text space. For
// every other kind it is a thousandth, which is what the format assumes.
func (f *Font) FontMatrix() [6]float64 { return f.fontMatrix }

// CharProcs are a Type 3 font's glyph drawings, by name.
func (f *Font) CharProcs() reader.Dict { return f.charProcs }

// Type3Resources are what a Type 3 font's glyph drawings draw with.
func (f *Font) Type3Resources() reader.Dict { return f.t3Res }

// Codes cuts a string into the codes it holds: one byte at a time for a simple
// font, two for a composite one, which is what every composite encoding seen in
// the wild uses.
func (f *Font) Codes(s []byte) []int {
	if f.kind != Composite {
		out := make([]int, len(s))
		for i, b := range s {
			out[i] = int(b)
		}
		return out
	}
	out := make([]int, 0, len(s)/2)
	for i := 0; i+1 < len(s); i += 2 {
		out = append(out, int(s[i])<<8|int(s[i+1]))
	}
	return out
}

// Width is how far the pen moves for one code, in text space — a multiple of
// the point size, so a width of half means half an em.
func (f *Font) Width(code int) float64 {
	if w, ok := f.widths[code]; ok {
		return w
	}
	return f.defaultW
}

// HasWidth reports whether the document said how wide this code is, rather
// than the width being the fallback [Font.Width] gives for one it did not.
//
// It matters for a font the document does not carry. One of the fourteen
// standard faces is written with no widths at all — every reader is expected
// to know Times and Helvetica and Courier by heart — so a reader drawing a
// stand-in has to take the advances from the stand-in too. Half an em for
// every letter reads as a typewriter, which is not what the page says.
func (f *Font) HasWidth(code int) bool {
	_, ok := f.widths[code]
	return ok
}

// GlyphName is what the document's encoding calls a code. ok is false for a
// composite font, which names nothing, and for a code the encoding passes over.
func (f *Font) GlyphName(code int) (string, bool) {
	name, ok := f.names[code]
	return name, ok
}

// Text is what a code stands for as characters: what the font's ToUnicode map
// says, or failing that what its glyph name says. ok is false when neither
// does — a subsetted font naming its glyphs g17 and carrying no map cannot be
// read back, and saying so is better than guessing.
func (f *Font) Text(code int) (string, bool) {
	if s, ok := f.toUni[code]; ok && s != "" {
		return s, true
	}
	if name, ok := f.names[code]; ok && f.namedByTheDocument(code) {
		// Not one character: a name may say it is a ligature of several, and
		// a code that stands for two letters has to give back two.
		if text, ok := TextOfGlyphName(name); ok {
			return text, true
		}
	}
	if f.fallback != nil {
		if s, ok := f.fallback(code); ok && s != "" {
			return s, true
		}
	}
	return "", false
}

// SetFallback installs a last resort for codes the document says nothing
// about: what the embedded font program calls them.
//
// A font that says it is symbolic is addressed through its own character map,
// and what that map says is the only honest answer for a code the document did
// not name. Reading the program is another package's work, so a caller that
// can do it hands the answer in here.
func (f *Font) SetFallback(fn func(code int) (string, bool)) { f.fallback = fn }

// namedByTheDocument reports whether a code's name can be believed as a
// statement about which character it is.
//
// A font that says it is symbolic is addressed through its own character map,
// and the base encoding is then a guess. It is a bad one: a mathematical font
// puts a capital gamma where the standard encoding puts an inverted
// exclamation mark, and reading the name would give the wrong letter with
// nothing to say so. A name the document chose itself, in a Differences
// array, is a different matter — that one it meant.
func (f *Font) namedByTheDocument(code int) bool {
	return !f.symbolic || f.chosen[code]
}

// Chosen reports whether the document named this code itself rather than
// inheriting the name from a base encoding.
func (f *Font) Chosen(code int) bool { return f.chosen[code] }

// CIDToGID maps a character identifier to a glyph number for a composite font.
// ok is false when the font's map does not reach that far.
func (f *Font) CIDToGID(cid int) (int, bool) {
	if f.cidToGID == nil {
		return cid, true
	}
	i := cid * 2
	if i+1 >= len(f.cidToGID) {
		return 0, false
	}
	return int(f.cidToGID[i])<<8 | int(f.cidToGID[i+1]), true
}

// Program is the embedded font program: which key it arrived under, and its
// bytes. ok is false for a font whose program is not embedded, which is every
// standard face and many others besides.
func (f *Font) Program() (reader.Name, []byte, bool) {
	for _, key := range []reader.Name{"FontFile2", "FontFile3", "FontFile"} {
		stream, ok := reader.ToStream(resolve(f.doc, f.descriptor.Get(key)))
		if !ok {
			continue
		}
		data, img, err := f.doc.DecodeStream(stream)
		if err != nil || img != "" {
			continue
		}
		return key, data, true
	}
	return "", nil, false
}

// resolve follows an indirect reference. A document that opened cannot fail to
// resolve one, so there is nothing to handle at every use.
func resolve(d *reader.Document, o reader.Object) reader.Object {
	out, _ := d.Resolve(o)
	return out
}

// floatOf reads a number, or gives a default.
func floatOf(o reader.Object, def float64) float64 {
	if v, ok := reader.ToFloat(o); ok {
		return v
	}
	return def
}
