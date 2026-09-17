package pdf

import (
	"fmt"
	"strings"

	"github.com/xcreativs/terios/api/internal/domain/agreement"
)

// SignedAgreement renders one signed agreement as a PDF: the practice's
// header, the exact wording the signature was given against, and the
// execution block.
//
// The document is reproduced from the signature rather than stored beside
// it, and is byte-identical every time it is asked for, because every input
// is frozen: the agreement text is pinned to the version signed, the typed
// name is kept verbatim, and the timestamp does not move. There is nothing
// to keep in sync and nothing that can drift away from the record.
func SignedAgreement(a agreement.Agreement, sig agreement.Signature) []byte {
	blocks := []Block{
		{Text: "TERIOS WELLNESS SPA", Style: Bold, Size: 13, SpaceAfter: 4},
		{Text: "Holistic Health & Wellness Practice", Style: Italic, Size: 9, SpaceAfter: 20},
		Title(a.Title),
	}

	// The body's own shape is preserved: "## " marks a heading, a leading
	// number or bullet indents as a clause, everything else is a paragraph.
	for _, chunk := range strings.Split(a.Body, "\n\n") {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		switch {
		case strings.HasPrefix(chunk, "## "):
			blocks = append(blocks, Heading(strings.TrimPrefix(chunk, "## ")))
		case strings.HasPrefix(chunk, "# "):
			blocks = append(blocks, Heading(strings.TrimPrefix(chunk, "# ")))
		case isClause(chunk):
			blocks = append(blocks, Indented(chunk, 18))
		default:
			blocks = append(blocks, Paragraph(chunk))
		}
	}

	blocks = append(blocks,
		Spacer(18),
		Heading("Signatures & Execution"),
		Label("CLIENT SIGNATURE"),
		Block{Text: sig.SignedName, Style: Italic, Size: 15, SpaceAfter: 10},
		Label("CLIENT ON FILE"),
		Paragraph(clientLine(sig)),
		Label("SIGNED AT"),
		Paragraph(sig.SignedAt.Format("2 January 2006 at 15:04 MST")),
	)

	if sig.PractitionerSignedName != "" {
		blocks = append(blocks,
			Spacer(8),
			Label("PRACTITIONER / COACH COUNTERSIGNATURE"),
			Block{Text: sig.PractitionerSignedName, Style: Italic, Size: 15, SpaceAfter: 10},
		)
		if sig.PractitionerSignedAt != nil {
			blocks = append(blocks,
				Label("COUNTERSIGNED AT"),
				Paragraph(sig.PractitionerSignedAt.Format("2 January 2006 at 15:04 MST")),
			)
		}
	} else if agreement.RequiresPractitionerSignature(sig.AgreementKey) {
		blocks = append(blocks,
			Spacer(8),
			Label("PRACTITIONER / COACH COUNTERSIGNATURE"),
			Block{Text: "[Pending Practitioner Countersignature]", Style: Italic, Size: 12, SpaceAfter: 10},
		)
	}

	blocks = append(blocks,
		Spacer(10),
		Label("AGREEMENT VERSION"),
		Paragraph(fmt.Sprintf("Version %d", sig.AgreementVersion)),
		Spacer(10),
		Block{
			Text: "This agreement was accepted electronically. Signatures recorded above " +
				"were executed in italics per electronic signature formatting standards and " +
				"recorded against the version of the wording shown in this document.",
			Size: 9, SpaceAfter: 0,
		},
	)
	return Build(blocks)
}

func clientLine(sig agreement.Signature) string {
	if sig.ClientEmail == "" {
		return sig.ClientName
	}
	return fmt.Sprintf("%s (%s)", sig.ClientName, sig.ClientEmail)
}

// isClause reports whether a chunk reads as a numbered or bulleted item,
// which is set indented rather than flush with the margin.
func isClause(chunk string) bool {
	if strings.HasPrefix(chunk, "- ") || strings.HasPrefix(chunk, "* ") {
		return true
	}
	for i, r := range chunk {
		if r >= '0' && r <= '9' {
			continue
		}
		return i > 0 && (r == '.' || r == ')')
	}
	return false
}
