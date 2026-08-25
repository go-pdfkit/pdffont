package pdffont

import "github.com/go-pdfkit/reader"

// readSimple reads a font addressed one byte at a time: what each code is
// called, and how wide it is.
func (f *Font) readSimple() {
	f.descriptor, _ = f.doc.GetDict(f.dict, "FontDescriptor")
	f.symbolic = symbolicFlag(f.doc, f.descriptor)
	f.names = f.readEncoding()
	f.readSimpleWidths()
}

// readType3 reads a font whose glyphs are little drawings.
func (f *Font) readType3() {
	f.descriptor, _ = f.doc.GetDict(f.dict, "FontDescriptor")
	f.names = f.readEncoding()
	f.charProcs, _ = f.doc.GetDict(f.dict, "CharProcs")
	f.t3Res, _ = f.doc.GetDict(f.dict, "Resources")
	if m := floatArray(f.doc, f.dict.Get("FontMatrix")); len(m) >= 6 {
		copy(f.fontMatrix[:], m[:6])
	}
	f.readSimpleWidths()
	// A Type 3 font's widths are in its own glyph space, which the font
	// matrix maps onto text space.
	for code, w := range f.widths {
		f.widths[code] = w * 1000 * f.fontMatrix[0]
	}
	f.defaultW = 0
}

// symbolicFlag reads whether a font says it is addressed through its own
// character map rather than through an encoding of names.
func symbolicFlag(d *reader.Document, descriptor reader.Dict) bool {
	flags, ok := reader.ToInt(resolve(d, descriptor.Get("Flags")))
	if !ok {
		return false
	}
	const symbolic, nonSymbolic = 1 << 2, 1 << 5
	return flags&symbolic != 0 && flags&nonSymbolic == 0
}

// readEncoding works out what each code of a simple font is called: a base
// table, and then whatever differences the font names on top of it.
func (f *Font) readEncoding() map[int]string {
	base := StandardEncoding
	out := map[int]string{}
	f.chosen = map[int]bool{}
	enc := resolve(f.doc, f.dict.Get("Encoding"))
	if name, ok := reader.ToName(enc); ok {
		table := NamedEncoding(name, base)
		fillNames(out, table)
		// A named encoding is the document's own word on what its codes are,
		// symbolic font or not.
		for code, n := range table {
			if n != "" {
				f.chosen[code] = true
			}
		}
		return out
	}
	encDict, ok := reader.ToDict(enc)
	if !ok {
		fillNames(out, base)
		return out
	}
	if name, ok := reader.ToName(resolve(f.doc, encDict.Get("BaseEncoding"))); ok {
		base = NamedEncoding(name, base)
	}
	fillNames(out, base)
	f.applyDifferences(out, encDict)
	return out
}

// NamedEncoding is the table a name stands for, or the fallback for a name
// that is not one of them.
func NamedEncoding(name reader.Name, fallback [256]string) [256]string {
	switch name {
	case "WinAnsiEncoding":
		return WinAnsiEncoding
	case "StandardEncoding", "MacRomanEncoding", "MacExpertEncoding":
		// Mac Roman differs from the standard encoding above code 127, where
		// almost nothing in a Latin document lives; using the standard table
		// is closer than using none.
		return StandardEncoding
	}
	return fallback
}

// fillNames copies a table into the map the rest of this works from.
func fillNames(out map[int]string, table [256]string) {
	for code, name := range table {
		if name != "" {
			out[code] = name
		}
	}
}

// applyDifferences reads the array that names glyphs a code at a time: a
// number, then the names of the codes from there on.
func (f *Font) applyDifferences(out map[int]string, encDict reader.Dict) {
	arr, ok := reader.ToArray(resolve(f.doc, encDict.Get("Differences")))
	if !ok {
		return
	}
	code := 0
	for _, e := range arr {
		v := resolve(f.doc, e)
		if n, ok := reader.ToInt(v); ok {
			code = int(n)
			continue
		}
		if name, ok := reader.ToName(v); ok {
			if code >= 0 && code < 256 {
				out[code] = string(name)
				f.chosen[code] = true
			}
			code++
		}
	}
}

// readSimpleWidths reads /Widths, which gives one width per code from
// /FirstChar on.
func (f *Font) readSimpleWidths() {
	first, ok := reader.ToInt(resolve(f.doc, f.dict.Get("FirstChar")))
	if !ok {
		return
	}
	arr, ok := reader.ToArray(resolve(f.doc, f.dict.Get("Widths")))
	if !ok {
		return
	}
	for i, e := range arr {
		if v, ok := reader.ToFloat(resolve(f.doc, e)); ok {
			f.widths[int(first)+i] = v / 1000
		}
	}
	if mw := floatOf(resolve(f.doc, f.descriptor.Get("MissingWidth")), -1); mw >= 0 {
		f.defaultW = mw / 1000
	}
}

// readComposite reads a font addressed by character identifier: its descendant
// holds the widths, the descriptor and the map from identifier to glyph.
func (f *Font) readComposite() {
	f.defaultW = 1
	arr, ok := reader.ToArray(resolve(f.doc, f.dict.Get("DescendantFonts")))
	if !ok || len(arr) == 0 {
		return
	}
	kid, ok := reader.ToDict(resolve(f.doc, arr[0]))
	if !ok {
		return
	}
	if v, ok := reader.ToFloat(resolve(f.doc, kid.Get("DW"))); ok {
		f.defaultW = v / 1000
	}
	f.readCIDWidths(kid)
	f.descriptor, _ = f.doc.GetDict(kid, "FontDescriptor")
	f.symbolic = symbolicFlag(f.doc, f.descriptor)
	if stream, ok := reader.ToStream(resolve(f.doc, kid.Get("CIDToGIDMap"))); ok {
		if data, img, err := f.doc.DecodeStream(stream); err == nil && img == "" {
			f.cidToGID = data
		}
	}
}

// readCIDWidths reads the /W array, which names widths one identifier at a
// time or a run at a time.
func (f *Font) readCIDWidths(kid reader.Dict) {
	arr, ok := reader.ToArray(resolve(f.doc, kid.Get("W")))
	if !ok {
		return
	}
	for i := 0; i < len(arr); {
		first, ok := reader.ToInt(resolve(f.doc, arr[i]))
		if !ok || i+1 >= len(arr) {
			return
		}
		next := resolve(f.doc, arr[i+1])
		if list, ok := reader.ToArray(next); ok {
			for k, e := range list {
				if v, ok := reader.ToFloat(resolve(f.doc, e)); ok {
					f.widths[int(first)+k] = v / 1000
				}
			}
			i += 2
			continue
		}
		last, ok := reader.ToInt(next)
		if !ok || i+2 >= len(arr) {
			return
		}
		w, ok := reader.ToFloat(resolve(f.doc, arr[i+2]))
		if !ok {
			return
		}
		if last >= first && last-first < 1<<20 {
			for c := first; c <= last; c++ {
				f.widths[int(c)] = w / 1000
			}
		}
		i += 3
	}
}

// floatArray reads an array of numbers, or nothing.
func floatArray(d *reader.Document, o reader.Object) []float64 {
	arr, ok := reader.ToArray(resolve(d, o))
	if !ok {
		return nil
	}
	out := make([]float64, 0, len(arr))
	for _, e := range arr {
		v, ok := reader.ToFloat(resolve(d, e))
		if !ok {
			return nil
		}
		out = append(out, v)
	}
	return out
}
