# pdffont

[![CI](https://github.com/go-pdfkit/pdffont/actions/workflows/ci.yml/badge.svg)](https://github.com/go-pdfkit/pdffont/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-pdfkit/pdffont.svg)](https://pkg.go.dev/github.com/go-pdfkit/pdffont)
[![License: BSD-3-Clause](https://img.shields.io/badge/License-BSD--3--Clause-blue.svg)](LICENSE)
[![Coverage](https://img.shields.io/badge/coverage-100%25-brightgreen.svg)](#how-it-is-checked)

The PDF side of a font: not the outlines, which are the font program's
business, but everything the document itself says about how a string of
bytes is to be read.

Which codes a string holds, how wide each one is, what each is called, and
what text each stands for. It is what a renderer needs before it can ask a
font program for a glyph, and all an extractor needs to read a page back as
words — so neither has to say it twice.

Pure Go, no C, nothing outside [`reader`](https://github.com/go-pdfkit/reader)
and the standard library.

## What it reads

Simple fonts, addressed one byte at a time through a base encoding and
whatever differences the document names on top of it. Composite fonts,
addressed by character identifier, with their widths and their map from
identifier to glyph. Type 3 fonts, whose glyphs are little drawings and
whose widths are in a space of their own.

And `/ToUnicode`, which is the one thing in a PDF that says what its text
actually **says**. Without it a page can be drawn but not read.

## What it refuses to guess

A font that says it is **symbolic** is addressed through its own character
map, and a base encoding is a guess at it — a bad one. A mathematical font
puts a capital gamma where the standard encoding puts an inverted
exclamation mark, so reading the name gives the wrong letter with nothing
to say so.

Such a code is left unread unless the document named it itself, or unless a
caller installs [`Font.SetFallback`](https://pkg.go.dev/github.com/go-pdfkit/pdffont#Font.SetFallback)
to say what the embedded program calls it. Reading the program is another
package's work; `extract` does it with
[`go-opentype`](https://github.com/go-opentype/opentype).

## How it is checked

100% of statements, including every branch that handles a dictionary saying
something it should not.

And the glyph-name table — which is the one piece of data here, and where a
single wrong entry would silently give the wrong letter — is held up
against the documents themselves. A font that both names a code and carries
a `/ToUnicode` entry for it states the same fact twice, so the two can be
compared. Over 12 000 corpus files: **63 515 names checked, 63 074 agreed —
99.31%**. The disagreements are producers whose own maps are wrong
(`alpha` → `↵`, `ff` → `!`), plus `Delta` and `Omega`, which Adobe's list
gives as the increment and ohm signs; the Greek letters written here are
the same characters by Unicode's own equivalence, and the ones a person
searching would type.

## The rest of the family

[**reader**](https://github.com/go-pdfkit/reader) reads and writes the format ·
[**ops**](https://github.com/go-pdfkit/ops) is the verbs and the `pdfops` command ·
[**render**](https://github.com/go-pdfkit/render) turns a page into pixels ·
[**pdffont**](https://github.com/go-pdfkit/pdffont) is what a document says about a font ·
[**extract**](https://github.com/go-pdfkit/extract) reads a page back ·
[**coedit**](https://github.com/go-pdfkit/coedit) is a PDF several people edit at once ·
[**app**](https://github.com/go-pdfkit/app) is the browser workbench.

Documentation for all of it: <https://go-pdfkit.github.io/docs/>

## Licence

BSD-3-Clause.
