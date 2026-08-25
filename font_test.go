package pdffont

import (
	"testing"

	"github.com/go-pdfkit/reader"
)

func TestASimpleFont(t *testing.T) {
	f := fontDoc(t, func(w *reader.Writer) reader.Dict {
		return reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
			"BaseFont": reader.Name("Helvetica"), "FirstChar": reader.Integer(65),
			"LastChar": reader.Integer(67), "Widths": widthArray(600, 700, 800),
			"Encoding": reader.Name("WinAnsiEncoding"),
		}
	})
	if f.Kind() != Simple {
		t.Errorf("Kind() = %v", f.Kind())
	}
	if got := f.Codes([]byte("ABC")); len(got) != 3 || got[0] != 'A' {
		t.Errorf("Codes() = %v", got)
	}
	for code, want := range map[int]float64{65: 0.6, 66: 0.7, 67: 0.8, 90: 0.5} {
		if got := f.Width(code); got != want {
			t.Errorf("Width(%d) = %v, want %v", code, got, want)
		}
	}
	if name, ok := f.GlyphName('A'); !ok || name != "A" {
		t.Errorf("GlyphName('A') = %q (%v)", name, ok)
	}
	if _, ok := f.GlyphName(1); ok {
		t.Error("a code the encoding passes over has a name")
	}
	if s, ok := f.Text('A'); !ok || s != "A" {
		t.Errorf("Text('A') = %q (%v)", s, ok)
	}
	if f.Symbolic() {
		t.Error("a font with no flags says it is symbolic")
	}
	if _, _, ok := f.Program(); ok {
		t.Error("a font with no program says it has one")
	}
	if got := f.FontMatrix(); got[0] != 0.001 {
		t.Errorf("FontMatrix() = %v", got)
	}
	if f.Dict() == nil {
		t.Error("the dictionary is not there")
	}
}

func TestAFontWithAMissingWidth(t *testing.T) {
	f := fontDoc(t, func(w *reader.Writer) reader.Dict {
		return reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
			"FirstChar": reader.Integer(65), "LastChar": reader.Integer(65),
			"Widths": widthArray(600),
			"FontDescriptor": w.Add(reader.Dict{
				"Type": reader.Name("FontDescriptor"), "MissingWidth": reader.Integer(250)}),
		}
	})
	if got := f.Width(90); got != 0.25 {
		t.Errorf("a code with no width of its own is %v wide", got)
	}
}

func TestAFontWithNamesOfItsOwn(t *testing.T) {
	f := fontDoc(t, func(w *reader.Writer) reader.Dict {
		return reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
			"Encoding": w.Add(reader.Dict{
				"Type": reader.Name("Encoding"), "BaseEncoding": reader.Name("WinAnsiEncoding"),
				"Differences": reader.Array{
					reader.Integer(1), reader.Name("alpha"), reader.Name("beta"),
					reader.Integer(200), reader.Name("Omega"),
					reader.Integer(300), reader.Name("outside"),
					reader.Integer(3), reader.Integer(4), // a number where a name should be
				}}),
		}
	})
	for code, want := range map[int]string{1: "α", 2: "β", 200: "Ω"} {
		if got, ok := f.Text(code); !ok || got != want {
			t.Errorf("Text(%d) = %q (%v), want %q", code, got, ok, want)
		}
	}
	if !f.Chosen(1) {
		t.Error("a name the document chose says it did not")
	}
	if f.Chosen(70) {
		t.Error("a name inherited from a base encoding says the document chose it")
	}
	if _, ok := f.GlyphName(300); ok {
		t.Error("a code outside a byte was named")
	}
}

func TestASymbolicFontIsNotReadThroughAGuess(t *testing.T) {
	// A font that says it is symbolic is addressed through its own character
	// map. Reading its codes through the standard encoding gives the wrong
	// letter with nothing to say so — a mathematical font puts a capital
	// gamma where that encoding puts an inverted exclamation mark.
	f := fontDoc(t, func(w *reader.Writer) reader.Dict {
		return reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
			"FontDescriptor": w.Add(reader.Dict{"Flags": reader.Integer(4)}),
		}
	})
	if !f.Symbolic() {
		t.Fatal("the font does not say it is symbolic")
	}
	if s, ok := f.Text(0xA1); ok {
		t.Errorf("a symbolic font's unnamed code was read as %q", s)
	}
	// Until the program says what it is.
	f.SetFallback(func(code int) (string, bool) {
		if code == 0xA1 {
			return "Γ", true
		}
		return "", false
	})
	if s, ok := f.Text(0xA1); !ok || s != "Γ" {
		t.Errorf("with the program consulted it is %q (%v)", s, ok)
	}
	if _, ok := f.Text(0xA2); ok {
		t.Error("a code the program does not name was read anyway")
	}
	// A name the document chose itself is believed even so.
	f2 := fontDoc(t, func(w *reader.Writer) reader.Dict {
		return reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
			"FontDescriptor": w.Add(reader.Dict{"Flags": reader.Integer(4)}),
			"Encoding": w.Add(reader.Dict{"Differences": reader.Array{
				reader.Integer(0xA1), reader.Name("Gamma")}}),
		}
	})
	if s, ok := f2.Text(0xA1); !ok || s != "Γ" {
		t.Errorf("a name the document chose came back as %q (%v)", s, ok)
	}
	// And a font that says it is both symbolic and not is taken at the
	// second word, since that is what a producer writing both means.
	f3 := fontDoc(t, func(w *reader.Writer) reader.Dict {
		return reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
			"FontDescriptor": w.Add(reader.Dict{"Flags": reader.Integer(4 | 32)}),
		}
	})
	if f3.Symbolic() {
		t.Error("a font saying both was taken as symbolic")
	}
	// A descriptor whose flags are not a number says nothing.
	f4 := fontDoc(t, func(w *reader.Writer) reader.Dict {
		return reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
			"FontDescriptor": w.Add(reader.Dict{"Flags": reader.Name("many")}),
		}
	})
	if f4.Symbolic() {
		t.Error("flags that are not a number made a font symbolic")
	}
}

func TestACompositeFont(t *testing.T) {
	f := fontDoc(t, func(w *reader.Writer) reader.Dict {
		kid := w.Add(reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("CIDFontType2"),
			"DW": reader.Integer(1000),
			"W": reader.Array{
				reader.Integer(1), reader.Array{reader.Integer(500), reader.Integer(600)},
				reader.Integer(10), reader.Integer(12), reader.Integer(700),
			},
			"CIDToGIDMap": w.Add(&reader.Stream{Dict: reader.Dict{},
				Raw: []byte{0, 0, 0, 5, 0, 9}}),
		})
		return reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type0"),
			"Encoding": reader.Name("Identity-H"), "DescendantFonts": reader.Array{kid},
		}
	})
	if f.Kind() != Composite {
		t.Errorf("Kind() = %v", f.Kind())
	}
	if got := f.Codes([]byte{0, 65, 1, 2}); len(got) != 2 || got[0] != 65 || got[1] != 258 {
		t.Errorf("Codes() = %v", got)
	}
	for code, want := range map[int]float64{1: 0.5, 2: 0.6, 10: 0.7, 12: 0.7, 99: 1} {
		if got := f.Width(code); got != want {
			t.Errorf("Width(%d) = %v, want %v", code, got, want)
		}
	}
	for cid, want := range map[int]int{0: 0, 1: 5, 2: 9} {
		got, ok := f.CIDToGID(cid)
		if !ok || got != want {
			t.Errorf("CIDToGID(%d) = %d (%v), want %d", cid, got, ok, want)
		}
	}
	if _, ok := f.CIDToGID(99); ok {
		t.Error("an identifier past the end of the map was mapped")
	}
	if _, ok := f.GlyphName(1); ok {
		t.Error("a composite font named a glyph")
	}
}

func TestACompositeFontWithNoMapOfItsOwn(t *testing.T) {
	f := fontDoc(t, func(w *reader.Writer) reader.Dict {
		kid := w.Add(reader.Dict{"Subtype": reader.Name("CIDFontType2")})
		return reader.Dict{"Subtype": reader.Name("Type0"),
			"DescendantFonts": reader.Array{kid}}
	})
	if got, ok := f.CIDToGID(7); !ok || got != 7 {
		t.Errorf("with no map an identifier is glyph %d (%v)", got, ok)
	}
	if got := f.Width(1); got != 1 {
		t.Errorf("with no widths a glyph is %v wide", got)
	}
}

func TestAType3Font(t *testing.T) {
	f := fontDoc(t, func(w *reader.Writer) reader.Dict {
		return reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type3"),
			"FontMatrix": reader.Array{reader.Real(0.01), reader.Integer(0), reader.Integer(0),
				reader.Real(0.01), reader.Integer(0), reader.Integer(0)},
			"FirstChar": reader.Integer(97), "LastChar": reader.Integer(97),
			"Widths":    widthArray(50),
			"CharProcs": reader.Dict{"a": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("0 0 m")})},
			"Resources": reader.Dict{"X": reader.Integer(1)},
			"Encoding": w.Add(reader.Dict{"Differences": reader.Array{
				reader.Integer(97), reader.Name("a")}}),
		}
	})
	if f.Kind() != Type3 {
		t.Errorf("Kind() = %v", f.Kind())
	}
	if got := f.FontMatrix(); got[0] != 0.01 {
		t.Errorf("FontMatrix() = %v", got)
	}
	// A Type 3 font's widths are in its own space, which its matrix maps on.
	if got := f.Width(97); got != 0.5 {
		t.Errorf("Width(97) = %v, want 0.5", got)
	}
	if got := f.Width(98); got != 0 {
		t.Errorf("a code with no width is %v wide", got)
	}
	if f.CharProcs() == nil {
		t.Error("the glyph drawings are not there")
	}
	if f.Type3Resources() == nil {
		t.Error("what the drawings draw with is not there")
	}
}

func TestFontDictionariesThatSayLittleOrNothing(t *testing.T) {
	cases := []struct {
		name  string
		build func(w *reader.Writer) reader.Dict
	}{
		{"nothing at all", func(w *reader.Writer) reader.Dict { return reader.Dict{} }},
		{"a composite font with no descendant", func(w *reader.Writer) reader.Dict {
			return reader.Dict{"Subtype": reader.Name("Type0")}
		}},
		{"a composite font whose descendant is not a dictionary", func(w *reader.Writer) reader.Dict {
			return reader.Dict{"Subtype": reader.Name("Type0"),
				"DescendantFonts": reader.Array{reader.Integer(3)}}
		}},
		{"widths with no first character", func(w *reader.Writer) reader.Dict {
			return reader.Dict{"Subtype": reader.Name("Type1"), "Widths": widthArray(600)}
		}},
		{"a first character with no widths", func(w *reader.Writer) reader.Dict {
			return reader.Dict{"Subtype": reader.Name("Type1"), "FirstChar": reader.Integer(65)}
		}},
		{"widths that are not numbers", func(w *reader.Writer) reader.Dict {
			return reader.Dict{"Subtype": reader.Name("Type1"), "FirstChar": reader.Integer(65),
				"Widths": reader.Array{reader.Name("wide")}}
		}},
		{"an encoding that is neither a name nor a dictionary", func(w *reader.Writer) reader.Dict {
			return reader.Dict{"Subtype": reader.Name("Type1"), "Encoding": reader.Integer(3)}
		}},
		{"an encoding naming a table nobody has", func(w *reader.Writer) reader.Dict {
			return reader.Dict{"Subtype": reader.Name("Type1"), "Encoding": reader.Name("Klingon")}
		}},
		{"differences that are not an array", func(w *reader.Writer) reader.Dict {
			return reader.Dict{"Subtype": reader.Name("Type1"),
				"Encoding": w.Add(reader.Dict{"Differences": reader.Integer(3)})}
		}},
		{"a Type 3 font with a matrix that is not one", func(w *reader.Writer) reader.Dict {
			return reader.Dict{"Subtype": reader.Name("Type3"),
				"FontMatrix": reader.Array{reader.Integer(1)}}
		}},
		{"a composite font whose widths are nonsense", func(w *reader.Writer) reader.Dict {
			kid := w.Add(reader.Dict{"W": reader.Array{reader.Name("x")}})
			return reader.Dict{"Subtype": reader.Name("Type0"), "DescendantFonts": reader.Array{kid}}
		}},
		{"a composite font whose widths stop halfway", func(w *reader.Writer) reader.Dict {
			kid := w.Add(reader.Dict{"W": reader.Array{reader.Integer(1)}})
			return reader.Dict{"Subtype": reader.Name("Type0"), "DescendantFonts": reader.Array{kid}}
		}},
		{"a composite font whose run has no width", func(w *reader.Writer) reader.Dict {
			kid := w.Add(reader.Dict{"W": reader.Array{reader.Integer(1), reader.Integer(2)}})
			return reader.Dict{"Subtype": reader.Name("Type0"), "DescendantFonts": reader.Array{kid}}
		}},
		{"a composite font whose run's width is not a number", func(w *reader.Writer) reader.Dict {
			kid := w.Add(reader.Dict{"W": reader.Array{reader.Integer(1), reader.Integer(2), reader.Name("x")}})
			return reader.Dict{"Subtype": reader.Name("Type0"), "DescendantFonts": reader.Array{kid}}
		}},
		{"a composite font whose run runs backwards", func(w *reader.Writer) reader.Dict {
			kid := w.Add(reader.Dict{"W": reader.Array{reader.Integer(9), reader.Integer(2), reader.Integer(500)}})
			return reader.Dict{"Subtype": reader.Name("Type0"), "DescendantFonts": reader.Array{kid}}
		}},
		{"a composite font whose map is filtered as an image", func(w *reader.Writer) reader.Dict {
			kid := w.Add(reader.Dict{"CIDToGIDMap": w.Add(&reader.Stream{
				Dict: reader.Dict{"Filter": reader.Name("DCTDecode")}, Raw: []byte("no")})})
			return reader.Dict{"Subtype": reader.Name("Type0"), "DescendantFonts": reader.Array{kid}}
		}},
	}
	for _, c := range cases {
		f := fontDoc(t, c.build)
		if f == nil {
			t.Errorf("%s: nothing came back", c.name)
			continue
		}
		// Whatever it says, a font can always be asked these.
		f.Codes([]byte("ab"))
		f.Width(65)
		f.Text(65)
		f.GlyphName(65)
		f.CIDToGID(1)
	}
}

func TestTheProgramAFontCarries(t *testing.T) {
	for _, key := range []reader.Name{"FontFile", "FontFile2", "FontFile3"} {
		f := fontDoc(t, func(w *reader.Writer) reader.Dict {
			return reader.Dict{"Subtype": reader.Name("Type1"),
				"FontDescriptor": w.Add(reader.Dict{
					key: w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("a program")})})}
		})
		got, data, ok := f.Program()
		if !ok || got != key || string(data) != "a program" {
			t.Errorf("%s: %q %q (%v)", key, got, data, ok)
		}
	}
	// One filtered as an image is not a program.
	f := fontDoc(t, func(w *reader.Writer) reader.Dict {
		return reader.Dict{"Subtype": reader.Name("Type1"),
			"FontDescriptor": w.Add(reader.Dict{
				"FontFile2": w.Add(&reader.Stream{
					Dict: reader.Dict{"Filter": reader.Name("DCTDecode")}, Raw: []byte("no")})})}
	})
	if _, _, ok := f.Program(); ok {
		t.Error("a program filtered as an image was handed over")
	}
	// And one that is not a stream at all.
	f = fontDoc(t, func(w *reader.Writer) reader.Dict {
		return reader.Dict{"Subtype": reader.Name("Type1"),
			"FontDescriptor": w.Add(reader.Dict{"FontFile": reader.Integer(3)})}
	})
	if _, _, ok := f.Program(); ok {
		t.Error("something that is not a stream was handed over as a program")
	}
	if f.Descriptor() == nil {
		t.Error("the descriptor is not there")
	}
}
