package pdffont

import (
	"testing"

	"github.com/go-pdfkit/reader"
)

func TestReadingAToUnicodeMap(t *testing.T) {
	// The two blocks that matter: one code at a time, and runs of them.
	m := ReadToUnicode([]byte(`
/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
2 beginbfchar
<0041> <0061>
<0042> <00620063>
endbfchar
2 beginbfrange
<0050> <0052> <0070>
<0060> <0062> [<0041> <0042> <0043>]
endbfrange
endcmap
`))
	want := map[int]string{
		0x41: "a", 0x42: "bc",
		0x50: "p", 0x51: "q", 0x52: "r",
		0x60: "A", 0x61: "B", 0x62: "C",
	}
	for code, w := range want {
		if got := m[code]; got != w {
			t.Errorf("%#x: %q, want %q", code, got, w)
		}
	}
	if len(m) != len(want) {
		t.Errorf("the map holds %d entries, want %d", len(m), len(want))
	}
}

func TestAToUnicodeMapOfCharactersOutsideThePlane(t *testing.T) {
	// A character past sixty-five thousand is written as two units, which is
	// how the format carries one at all.
	m := ReadToUnicode([]byte("beginbfchar <01> <D83DDE00> endbfchar"))
	if got := m[1]; got != "\U0001F600" {
		t.Errorf("%q", got)
	}
}

func TestToUnicodeMapsThatAreNotOnes(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"nothing at all", ""},
		{"no blocks", "begincmap endcmap"},
		{"a bfchar block that stops halfway", "beginbfchar <41>"},
		{"a bfchar whose value is not hexadecimal", "beginbfchar <41> named endbfchar"},
		{"a bfchar whose code is not hexadecimal", "beginbfchar named <41> endbfchar"},
		{"a bfrange that stops halfway", "beginbfrange <41> <43>"},
		{"a bfrange whose ends are not hexadecimal", "beginbfrange named <43> <61> endbfrange"},
		{"a bfrange running backwards", "beginbfrange <43> <41> <61> endbfrange"},
		{"a bfrange wider than any font", "beginbfrange <0000> <FFFFFF> <61> endbfrange"},
		{"a bfrange whose value is neither", "beginbfrange <41> <43> named endbfrange"},
		{"a bfrange with an empty value", "beginbfrange <41> <43> <> endbfrange"},
		{"a comment and nothing else", "% just a comment\n"},
	}
	for _, c := range cases {
		if m := ReadToUnicode([]byte(c.data)); len(m) != 0 {
			// A range with an empty value gives empty strings, which are not
			// text; everything else gives nothing at all.
			for _, v := range m {
				if v != "" {
					t.Errorf("%s gave %q", c.name, v)
				}
			}
		}
	}
}

func TestAToUnicodeMapWrittenLoosely(t *testing.T) {
	// The things a real producer does: an odd digit, a list shorter than the
	// range it belongs to, and a block that runs to the end of the stream.
	m := ReadToUnicode([]byte("beginbfchar <41> <061> endbfchar"))
	if len(m) != 1 {
		t.Errorf("an odd digit gave %v", m)
	}
	m = ReadToUnicode([]byte("beginbfrange <41> <45> [<0061> <0062>] endbfrange"))
	if m[0x41] != "a" || m[0x42] != "b" {
		t.Errorf("a short list gave %v", m)
	}
	if _, ok := m[0x43]; ok {
		t.Error("a code past the end of the list was named")
	}
	m = ReadToUnicode([]byte("beginbfchar <41> <0061>"))
	if m[0x41] != "a" {
		t.Errorf("a block running to the end gave %v", m)
	}
}

func TestAFontWhoseToUnicodeCannotBeRead(t *testing.T) {
	for _, c := range []struct {
		name  string
		build func(w *reader.Writer) reader.Dict
	}{
		{"not a stream", func(w *reader.Writer) reader.Dict {
			return reader.Dict{"Subtype": reader.Name("Type1"), "ToUnicode": reader.Integer(3)}
		}},
		{"filtered as an image", func(w *reader.Writer) reader.Dict {
			return reader.Dict{"Subtype": reader.Name("Type1"),
				"ToUnicode": w.Add(&reader.Stream{
					Dict: reader.Dict{"Filter": reader.Name("DCTDecode")}, Raw: []byte("no")})}
		}},
		{"saying nothing", func(w *reader.Writer) reader.Dict {
			return reader.Dict{"Subtype": reader.Name("Type1"),
				"ToUnicode": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("begincmap endcmap")})}
		}},
	} {
		f := fontDoc(t, c.build)
		if s, ok := f.Text('A'); !ok || s != "A" {
			t.Errorf("%s: the encoding should still have named it, got %q (%v)", c.name, s, ok)
		}
	}
	// One that does say something wins over the name.
	f := fontDoc(t, func(w *reader.Writer) reader.Dict {
		return reader.Dict{"Subtype": reader.Name("Type1"),
			"ToUnicode": w.Add(&reader.Stream{Dict: reader.Dict{},
				Raw: []byte("beginbfchar <41> <03B1> endbfchar")})}
	})
	if s, ok := f.Text('A'); !ok || s != "α" {
		t.Errorf("the map did not win: %q (%v)", s, ok)
	}
}

func TestTidyingWhatAMapSays(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"a", "a"},
		{"\x00", ""},
		{"a\x00", "a"},
		{"�", ""},
		{"�a�", "a"},
	} {
		if got := TrimText(c.in); got != c.want {
			t.Errorf("TrimText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTurningAGlyphNameIntoACharacter(t *testing.T) {
	// The three ways a name can say which character it is: the table, and
	// the two conventions the specification defines for naming one directly.
	cases := []struct {
		name string
		want rune
		ok   bool
	}{
		{"A", 'A', true},
		{"space", ' ', true},
		{"alpha", 'α', true},
		{"minus", '−', true},
		{"arrowdblboth", '⇔', true},
		{"uni0041", 'A', true},
		{"uni00E9", 'é', true},
		{"u1F600", '\U0001F600', true},
		{"u0041", 'A', true},
		{"g17", 0, false},
		{"", 0, false},
		{"uni00", 0, false},
		{"uniZZZZ", 0, false},
		{"u12", 0, false},
		{"uABCDEFGH", 0, false},
		{"index123", 0, false},
	}
	for _, c := range cases {
		got, ok := RuneOfGlyphName(c.name)
		if ok != c.ok {
			t.Errorf("%q: ok = %v, want %v", c.name, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("%q = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestTheEncodingsANameStandsFor(t *testing.T) {
	if got := NamedEncoding("WinAnsiEncoding", StandardEncoding); got[128] != "Euro" {
		t.Errorf("WinAnsi gives %q at 128", got[128])
	}
	for _, name := range []reader.Name{"StandardEncoding", "MacRomanEncoding", "MacExpertEncoding"} {
		if got := NamedEncoding(name, WinAnsiEncoding); got[39] != "quoteright" {
			t.Errorf("%s gives %q at 39", name, got[39])
		}
	}
	if got := NamedEncoding("Klingon", WinAnsiEncoding); got[128] != "Euro" {
		t.Error("a name nobody has heard of did not fall back")
	}
}

func TestAFontMatrixThatIsNotNumbers(t *testing.T) {
	f := fontDoc(t, func(w *reader.Writer) reader.Dict {
		return reader.Dict{"Subtype": reader.Name("Type3"),
			"FontMatrix": reader.Array{reader.Name("a"), reader.Integer(0), reader.Integer(0),
				reader.Name("b"), reader.Integer(0), reader.Integer(0)}}
	})
	if got := f.FontMatrix(); got[0] != 0.001 {
		t.Errorf("FontMatrix() = %v", got)
	}
}

func TestHexadecimalInEveryCase(t *testing.T) {
	// Both tables read hexadecimal, and a producer may write it either way
	// about.
	for _, name := range []string{"uni00e9", "uni00E9"} {
		if r, ok := RuneOfGlyphName(name); !ok || r != 'é' {
			t.Errorf("%q = %q (%v)", name, r, ok)
		}
	}
	m := ReadToUnicode([]byte("beginbfchar <41> <00e9> endbfchar"))
	if m[0x41] != "é" {
		t.Errorf("lower-case hexadecimal gave %q", m[0x41])
	}
	m = ReadToUnicode([]byte("beginbfchar <41> <00E9> endbfchar"))
	if m[0x41] != "é" {
		t.Errorf("upper-case hexadecimal gave %q", m[0x41])
	}
}

func TestABFRangeWrittenOddly(t *testing.T) {
	// A list that never closes, and a block that ends where it began.
	m := ReadToUnicode([]byte("beginbfrange <41> <43> [<0061> <0062>"))
	if m[0x41] != "a" {
		t.Errorf("an unclosed list gave %v", m)
	}
	if m := ReadToUnicode([]byte("beginbfrange endbfrange")); len(m) != 0 {
		t.Errorf("an empty range gave %v", m)
	}
	if m := ReadToUnicode([]byte("beginbfchar endbfchar")); len(m) != 0 {
		t.Errorf("an empty block gave %v", m)
	}
}

func TestWidthsThatAreNotAnArray(t *testing.T) {
	f := fontDoc(t, func(w *reader.Writer) reader.Dict {
		return reader.Dict{"Subtype": reader.Name("Type3"),
			"FontMatrix": reader.Integer(3)}
	})
	if got := f.FontMatrix(); got[0] != 0.001 {
		t.Errorf("FontMatrix() = %v", got)
	}
}

func TestABFRangeBlockThatEndsProperly(t *testing.T) {
	// A block that says where it ends, with a whole cmap around it, and one
	// whose list is longer than the range it belongs to.
	m := ReadToUnicode([]byte(`
begincmap
1 beginbfrange
<41> <42> <0061>
endbfrange
endcmap
end
end
`))
	if m[0x41] != "a" || m[0x42] != "b" {
		t.Errorf("gave %v", m)
	}
	m = ReadToUnicode([]byte("beginbfrange <41> <42> [<0061> <0062> <0063> <0064>] endbfrange endcmap end"))
	if m[0x41] != "a" || m[0x42] != "b" || len(m) != 2 {
		t.Errorf("a list longer than its range gave %v", m)
	}
}
