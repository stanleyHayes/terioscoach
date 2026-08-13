package document

import "errors"

// Domain errors for the documents slice.
var (
	// ErrDocumentNotFound means no document matches the lookup — including
	// one belonging to another client, which must be indistinguishable
	// from one that does not exist.
	ErrDocumentNotFound = errors.New("document not found")

	// Validation errors.
	ErrInvalidKind         = errors.New("unknown document kind")
	ErrClientRequired      = errors.New("a private document needs an owning client")
	ErrUploaderRequired    = errors.New("an uploader is required")
	ErrInvalidPublicID     = errors.New("a storage id is required")
	ErrInvalidFilename     = errors.New("a filename is required")
	ErrInvalidTitle        = errors.New("title is required")
	ErrUnsupportedFileType = errors.New("only pdf, jpg, png, and webp files are accepted")
	ErrFileTooLarge        = errors.New("file is too large")
)
