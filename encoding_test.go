package pdffont

import "testing"

func TestTheLettersOfTheLanguagesThatUseThem(t *testing.T) {
	// A document in Czech, Polish, Slovak, Hungarian, Turkish or Romanian is
	// set in glyphs with these names, and every one of them was being read as
	// nothing at all. A dropped character leaves no mark: "Příliš" comes back
	// "Píliš" and still looks like a word.
	for name, want := range map[string]rune{
		"rcaron": 'ř', "zdotaccent": 'ż', "Ccaron": 'Č', "ccaron": 'č',
		"nacute": 'ń', "aogonek": 'ą', "scedilla": 'ş', "gbreve": 'ğ',
		"sacute": 'ś', "cacute": 'ć', "Ecaron": 'Ě', "Lcaron": 'Ľ',
		"Uhungarumlaut": 'Ű', "Uring": 'Ů', "Tcaron": 'Ť', "Eng": 'Ŋ',
		"Abreve": 'Ă', "Lacute": 'Ĺ', "dcroat": 'đ', "IJ": 'Ĳ', "ij": 'ĳ',
		"Idotaccent": 'İ', "scommaaccent": 'ș', "kgreenlandic": 'ĸ',
		"longs": 'ſ', "napostrophe": 'ŉ', "hbar": 'ħ',
		"Tcedilla": 'Ţ', "tcedilla": 'ţ', "Germandbls": 'ẞ',
		"lscript": 'ℓ', "openbullet": '◦',
		"angbracketleft": '〈', "angbracketright": '〉',
		"tcommabelow": 'ț',
	} {
		got, ok := RuneOfGlyphName(name)
		if !ok || got != want {
			t.Errorf("%s read as %q %v, wanted %q", name, got, ok, want)
		}
	}
}

func TestAVariantOfACharacterIsStillThatCharacter(t *testing.T) {
	// A name may carry a variant after a full stop — a small capital, an
	// old-style figure, an alternate cut. Which cut it is does not change
	// which letter it is, and 7 272 names in the corpus are written this way.
	for name, want := range map[string]string{
		"a.sc": "a", "one.oldstyle": "1", "A.alt": "A",
		"eacute.sc": "é", "f.alt01": "f", "rcaron.ss01": "ř",
	} {
		got, ok := TextOfGlyphName(name)
		if !ok || got != want {
			t.Errorf("%s read as %q %v, wanted %q", name, got, ok, want)
		}
	}
}

func TestALigatureNamedByItsParts(t *testing.T) {
	// A name made of parts with underscores between them is those parts in
	// order. Without this, "Definition" comes back "Denition" — which is not
	// a missing character so much as a misspelt word.
	for name, want := range map[string]string{
		"f_i": "fi", "f_f": "ff", "f_f_i": "ffi", "f_l": "fl",
		"s_t": "st", "c_t": "ct", "a_b_c": "abc",
		"f_i.sc": "fi",
	} {
		got, ok := TextOfGlyphName(name)
		if !ok || got != want {
			t.Errorf("%s read as %q %v, wanted %q", name, got, ok, want)
		}
	}
	// A code standing for two letters gives back two, and a name asking for
	// that cannot be one character.
	if _, ok := RuneOfGlyphName("f_i"); ok {
		t.Error("a ligature of two letters was given back as one character")
	}
}

func TestANameThatSaysNothing(t *testing.T) {
	// A name of one character is that character, full stop and underscore
	// included — a glyph called "." is the full stop — so those are not here.
	for _, name := range []string{
		"", ".sc", "f_", "_i", "g17", "cid42", "index7",
		"nonesuch", "f_nonesuch", "uni", "uniZZZZ",
	} {
		if got, ok := TextOfGlyphName(name); ok {
			t.Errorf("%q was read as %q, and it says nothing about any character", name, got)
		}
	}
}

func TestTheConventionsForNamingACharacterOutright(t *testing.T) {
	for name, want := range map[string]rune{
		"uni0041": 'A', "uni00E9": 'é', "u0041": 'A', "u01D400": '\U0001D400',
		"A": 'A', "z": 'z',
	} {
		got, ok := RuneOfGlyphName(name)
		if !ok || got != want {
			t.Errorf("%s read as %q %v, wanted %q", name, got, ok, want)
		}
	}
}
