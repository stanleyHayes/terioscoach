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
	if sig.StatementOfWork != nil {
		answers := sig.StatementOfWork
		blocks := []Block{
			{Text: "TERIOS WELLNESS SPA", Style: Bold, Size: 13, SpaceAfter: 20},
			Title(a.Title), Label("CLIENT NAME"), Paragraph(answers.ClientName),
			Label(sowDateLabel(a.Key)), Paragraph(answers.EffectiveDate),
			Label("INITIAL TERM / PACKAGE"), Paragraph(sowTerm(answers)),
			Label("MONTHLY FEE"), Paragraph(answers.MonthlyFee),
		}
		if sig.SubmittedAt != nil {
			blocks = append(blocks, Spacer(18), Label("SUBMITTED AT"), Paragraph(sig.SubmittedAt.Format("2 January 2006 at 15:04 MST")))
		}
		if !sig.SignedAt.IsZero() {
			for _, paragraph := range strings.Split(sig.AgreementBody, "\n\n") {
				if strings.TrimSpace(paragraph) != "" {
					blocks = append(blocks, Paragraph(paragraph))
				}
			}
			blocks = append(blocks, executionBlocks(a.Key, sig)...)
			blocks = append(blocks, Label("DOCUMENT VERSION"), Paragraph(fmt.Sprintf("%d", sig.AgreementVersion)))
		} else {
			blocks = append(blocks, Paragraph("Submitted — unsigned (legacy)"))
		}
		return Build(blocks)
	}

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

	blocks = append(blocks, executionBlocks(a.Key, sig)...)

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

func executionBlocks(key string, sig agreement.Signature) []Block {
	blocks := []Block{Spacer(18), Heading("Signatures & Execution")}
	if sig.SignerRole == "guardian" {
		blocks = append(blocks, Label("PARTICIPANT"), Paragraph(sig.ParticipantName), Label("PARENT / GUARDIAN NAME"), Paragraph(sig.GuardianName), Label("PARENT / GUARDIAN SIGNATURE"), Block{Text: sig.SignedName, Style: Italic, Size: 15, SpaceAfter: 10}, Label("RELATIONSHIP"), Paragraph(sig.GuardianRelationship), Label("DECLARED CONTACT (NOT IDENTITY VERIFIED)"), Paragraph(sig.GuardianEmail), Label("CONSENT"), Paragraph(sig.ConsentBody), Label("CONSENT VERSION"), Paragraph(sig.ConsentVersion), Label("SIGNED AT (UTC)"), Paragraph(sig.SignedAt.UTC().Format("2006-01-02 15:04:05 MST")), Label("AUTHENTICATED ACTOR / CONTEXT"), Paragraph(sig.ActorID+" / "+sig.ContextID))
		if agreement.RequiresPractitionerSignature(key) {
			blocks = append(blocks, Label("PRACTITIONER SIGNATURE"), practitionerSignature(sig), Label("PRACTITIONER NAME"), Paragraph(practitionerName(sig)), Label("PRACTITIONER SIGNED AT"), Paragraph(practitionerDate(sig)))
		}
		return blocks
	}
	switch key {
	case "holistic_coaching", "nurse_coaching":
		blocks = append(blocks,
			Paragraph("IN WITNESS WHEREOF, the parties hereto have caused this Agreement to be executed as of the Effective Date by their respective duly authorized officers."),
			Label("TERIOS WELLNESS SPA"),
			Label("PRACTITIONER SIGNATURE"),
			practitionerSignature(sig),
			Label("NAME"),
			Paragraph(practitionerName(sig)),
			Label("DATE"),
			Paragraph(practitionerDate(sig)),
			Spacer(8),
			Label("CLIENT"),
			Label("CLIENT SIGNATURE"),
			Block{Text: sig.SignedName, Style: Italic, Size: 15, SpaceAfter: 10},
			Label("NAME"),
			Paragraph(clientLine(sig)),
			Label("DATE"),
			Paragraph(sig.SignedAt.Format("2 January 2006 at 15:04 MST")),
			guardianClause(),
		)
	default:
		blocks = append(blocks,
			Label("PRINTED NAME"),
			Paragraph(clientLine(sig)),
			Label("CLIENT SIGNATURE"),
			Block{Text: sig.SignedName, Style: Italic, Size: 15, SpaceAfter: 10},
			Label("DATE"),
			Paragraph(sig.SignedAt.Format("2 January 2006 at 15:04 MST")),
			guardianClause(),
		)
	}
	return blocks
}

func practitionerSignature(sig agreement.Signature) Block {
	if sig.PractitionerSignedName == "" {
		return Block{Text: "[Pending Practitioner Countersignature]", Style: Italic, Size: 12, SpaceAfter: 10}
	}
	return Block{Text: sig.PractitionerSignedName, Style: Italic, Size: 15, SpaceAfter: 10}
}

func practitionerName(sig agreement.Signature) string {
	if sig.PractitionerSignedName == "" {
		return "[Pending Practitioner Countersignature]"
	}
	return sig.PractitionerSignedName
}

func practitionerDate(sig agreement.Signature) string {
	if sig.PractitionerSignedAt == nil {
		return "[Pending Practitioner Countersignature]"
	}
	return sig.PractitionerSignedAt.Format("2 January 2006 at 15:04 MST")
}

func guardianClause() Block {
	return Paragraph("OR I am the parent or legal guardian of the minor named above. I have the legal right to consent to and, by signing below, I consent to the terms and conditions of this Release. Name: ____________________  Signature of parent or legal guardian, if under 18: ____________________  Date: ____________________")
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

func sowTerm(a *agreement.StatementOfWork) string {
	if a.Package != "" {
		return a.Package
	}
	return fmt.Sprintf("%d months", a.InitialTermMonths)
}

func sowDateLabel(key string) string {
	if key == "nurse_sow" {
		return "SERVICE START DATE"
	}
	return "EFFECTIVE DATE"
}
