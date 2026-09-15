// Package pdf writes the one kind of PDF this system needs: a plain,
// multi-page text document on A4, set in Helvetica.
//
// It is deliberately not a PDF library. A signed agreement is paragraphs,
// headings and a signature block — no images, tables, colour or vector
// work — and the whole of that fits in one readable file with no third-party
// dependency in the tree and nothing to keep up to date. Anything richer
// than this belongs in a real library, not in here.
//
// The output targets PDF 1.4 with WinAnsiEncoding, which every reader and
// every browser's built-in viewer opens without complaint.
package pdf

import (
	"bytes"
	"fmt"
	"strings"
)

// Page geometry, in PDF points (72 per inch). A4 with a 56pt margin.
const (
	pageWidth  = 595.28
	pageHeight = 841.89
	margin     = 56.0
	textWidth  = pageWidth - 2*margin
)

// Style is one of the faces a line can be set in.
type Style int

const (
	Regular Style = iota
	Bold
	Italic
)

func (s Style) fontRef() string {
	switch s {
	case Bold:
		return "/F2"
	case Italic:
		return "/F3"
	default:
		return "/F1"
	}
}

// Block is one run of text to lay out: a heading, a paragraph, or a spacer.
type Block struct {
	Text  string
	Style Style
	Size  float64
	// SpaceAfter is the gap below the block, in points.
	SpaceAfter float64
	// Indent shifts the block right, for list items and nested clauses.
	Indent float64
}

// Heading, Paragraph and Spacer build the blocks a document is made of.
func Heading(text string) Block {
	return Block{Text: text, Style: Bold, Size: 13, SpaceAfter: 8}
}

func Title(text string) Block {
	return Block{Text: text, Style: Bold, Size: 18, SpaceAfter: 16}
}

func Paragraph(text string) Block {
	return Block{Text: text, Size: 10.5, SpaceAfter: 9}
}

func Indented(text string, indent float64) Block {
	return Block{Text: text, Size: 10.5, SpaceAfter: 7, Indent: indent}
}

func Label(text string) Block {
	return Block{Text: text, Style: Bold, Size: 9, SpaceAfter: 3}
}

func Spacer(points float64) Block {
	return Block{SpaceAfter: points}
}

// Build renders blocks to a complete PDF file.
func Build(blocks []Block) []byte {
	lines := layout(blocks)
	pages := paginate(lines)
	return write(pages)
}

// placedLine is one line of text with its position resolved.
type placedLine struct {
	text   string
	style  Style
	size   float64
	indent float64
	// gapAfter is the leading plus any block spacing below this line.
	gapAfter float64
}

// layout turns blocks into a flat run of lines, wrapping each block to the
// text column.
func layout(blocks []Block) []placedLine {
	var out []placedLine
	for _, block := range blocks {
		if strings.TrimSpace(block.Text) == "" {
			// A spacer still consumes vertical space; it just has no glyphs.
			out = append(out, placedLine{gapAfter: block.SpaceAfter})
			continue
		}
		size := block.Size
		if size == 0 {
			size = 10.5
		}
		leading := size * 1.42
		wrapped := wrap(block.Text, block.Style, size, textWidth-block.Indent)
		for i, text := range wrapped {
			gap := leading
			if i == len(wrapped)-1 {
				gap += block.SpaceAfter
			}
			out = append(out, placedLine{
				text:     text,
				style:    block.Style,
				size:     size,
				indent:   block.Indent,
				gapAfter: gap,
			})
		}
	}
	return out
}

// paginate breaks lines into pages at the bottom margin.
func paginate(lines []placedLine) [][]placedLine {
	var pages [][]placedLine
	var current []placedLine
	remaining := pageHeight - 2*margin

	for _, line := range lines {
		if line.gapAfter > remaining && len(current) > 0 {
			pages = append(pages, current)
			current = nil
			remaining = pageHeight - 2*margin
		}
		current = append(current, line)
		remaining -= line.gapAfter
	}
	if len(current) > 0 {
		pages = append(pages, current)
	}
	if len(pages) == 0 {
		pages = [][]placedLine{{}}
	}
	return pages
}

// wrap breaks text into lines that fit within width.
//
// A single word longer than the column (a pasted URL, say) is broken at the
// column rather than allowed to run off the page: an unreadable line is
// better than an invisible one.
func wrap(text string, style Style, size, width float64) []string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		line := ""
		for _, word := range words {
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if textLen(candidate, style, size) <= width {
				line = candidate
				continue
			}
			if line != "" {
				lines = append(lines, line)
				line = ""
			}
			for textLen(word, style, size) > width {
				cut := len(word)
				for cut > 1 && textLen(word[:cut], style, size) > width {
					cut--
				}
				lines = append(lines, word[:cut])
				word = word[cut:]
			}
			line = word
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// textLen is the rendered width of s in points.
func textLen(s string, style Style, size float64) float64 {
	table := helvetica
	if style == Bold {
		table = helveticaBold
	}
	var total float64
	for _, b := range []byte(encode(s)) {
		switch {
		case b >= 32 && b <= 126:
			total += float64(table[b-32])
		default:
			if w, ok := highWidths[b]; ok {
				total += float64(w)
			} else {
				total += 556
			}
		}
	}
	return total * size / 1000
}

// winAnsi maps the typographic characters our copy actually uses onto their
// WinAnsiEncoding byte. Anything not listed and outside Latin-1 becomes
// "?": a visible stand-in beats a mojibake pair or a silent deletion.
var winAnsi = map[rune]byte{
	'\u2014': 0x97, // em dash
	'\u2013': 0x96, // en dash
	'\u2018': 0x91, // left single quote
	'\u2019': 0x92, // right single quote
	'\u201C': 0x93, // left double quote
	'\u201D': 0x94, // right double quote
	'\u2026': 0x85, // ellipsis
	'\u2022': 0x95, // bullet
	'\u2020': 0x86, // dagger
	'\u20AC': 0x80, // euro
	'\u00A0': 0x20, // non-breaking space, flattened
}

// highWidths are Helvetica advance widths for the mapped high bytes, so a
// line full of em dashes still wraps where it should.
var highWidths = map[byte]int{
	0x97: 1000, 0x96: 556, 0x91: 222, 0x92: 222,
	0x93: 333, 0x94: 333, 0x85: 1000, 0x95: 350,
	0x86: 556, 0x80: 556,
}

// encode maps a Go string onto WinAnsi bytes.
func encode(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteByte(' ')
		case r < 128:
			b.WriteByte(byte(r))
		default:
			if mapped, ok := winAnsi[r]; ok {
				b.WriteByte(mapped)
			} else if r < 256 {
				// Latin-1 and WinAnsi agree from 0xA0 up.
				b.WriteByte(byte(r))
			} else {
				b.WriteByte('?')
			}
		}
	}
	return b.String()
}

// escape prepares a string for a PDF literal.
func escape(s string) string {
	var b strings.Builder
	for _, c := range []byte(encode(s)) {
		switch c {
		case '(', ')', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\r', '\n':
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// content builds one page's content stream.
func content(lines []placedLine) string {
	var b strings.Builder
	b.WriteString("BT\n")
	y := pageHeight - margin
	for _, line := range lines {
		if line.text != "" {
			fmt.Fprintf(&b, "%s %.2f Tf\n", line.style.fontRef(), line.size)
			fmt.Fprintf(&b, "1 0 0 1 %.2f %.2f Tm\n", margin+line.indent, y-line.size)
			fmt.Fprintf(&b, "(%s) Tj\n", escape(line.text))
		}
		y -= line.gapAfter
	}
	b.WriteString("ET")
	return b.String()
}

// write assembles the object graph, the cross-reference table and the
// trailer into the finished file.
func write(pages [][]placedLine) []byte {
	// Object numbering: 1 catalog, 2 pages, 3, 4, and 5 fonts, then a
	// page object and a content stream for each page.
	const firstPageObj = 6
	total := 5 + 2*len(pages)

	var buf bytes.Buffer
	offsets := make([]int, total+1)
	buf.WriteString("%PDF-1.4\n")
	// A binary comment marks the file as containing 8-bit data, so a
	// transfer that might mangle line endings is treated as binary.
	buf.WriteString("%\xE2\xE3\xCF\xD3\n")

	object := func(number int, body string) {
		offsets[number] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", number, body)
	}

	object(1, "<< /Type /Catalog /Pages 2 0 R >>")

	var kids strings.Builder
	for i := range pages {
		fmt.Fprintf(&kids, "%d 0 R ", firstPageObj+2*i)
	}
	object(2, fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", len(pages), strings.TrimSpace(kids.String())))
	object(3, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	object(4, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	object(5, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Oblique /Encoding /WinAnsiEncoding >>")

	for i, page := range pages {
		pageObj := firstPageObj + 2*i
		streamObj := pageObj + 1
		object(pageObj, fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] "+
				"/Resources << /Font << /F1 3 0 R /F2 4 0 R /F3 5 0 R >> >> /Contents %d 0 R >>",
			pageWidth, pageHeight, streamObj))

		stream := content(page)
		object(streamObj, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	}

	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", total+1)
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= total; i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", total+1, xref)
	return buf.Bytes()
}

// Helvetica and Helvetica-Bold advance widths for ASCII 32-126, in 1/1000
// em — the Adobe standard metrics. Wrapping without them would either
// overflow the column or waste a third of it.
var helvetica = [95]int{
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584,
}

var helveticaBold = [95]int{
	278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 333, 333, 584, 584, 584, 611,
	975, 722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 333, 278, 333, 584, 556,
	333, 556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611,
	611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500, 389, 280, 389, 584,
}
