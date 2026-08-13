package ports

import (
	"context"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/note"
)

// NoteContent is the writable content of a session note (PUT semantics —
// all three fields are replaced wholesale).
type NoteContent struct {
	PrivateNotes    string
	SharedFeedback  string
	SharedResources []string
}

// NoteView is the role-dependent result of reading a booking's notes.
// Exactly one field is set: Full for the practitioner (everything,
// including private notes), Shared for the client (shared content only,
// after sharing — private notes are filtered out by the domain before they
// ever reach the transport).
type NoteView struct {
	Full   *note.SessionNote
	Shared *note.SharedContent
}

// ShareResult carries the note after a share call plus the transition
// signal: JustShared is true only when this call performed the share (the
// first one). The notifications slice (BE-09) keys the feedback email off
// it — repeat calls must not re-send.
type ShareResult struct {
	Note       note.SessionNote
	JustShared bool
}

// NoteService is the inbound port for the session-notes slice.
type NoteService interface {
	// GetNotes returns a booking's notes. The practitioner sees the full
	// note; the owning client sees the shared content only — an unshared
	// note returns note.ErrNoteNotFound, indistinguishable from no note.
	// Cross-owner access returns booking.ErrBookingNotFound (isolation).
	GetNotes(ctx context.Context, id identity.Identity, bookingID string) (NoteView, error)
	// UpsertNotes creates or replaces the note's content on one of the
	// practitioner's bookings. One note per booking; sharedAt is untouched
	// by content edits. Cross-practitioner access returns
	// booking.ErrBookingNotFound.
	UpsertNotes(ctx context.Context, practitionerID, bookingID string, content NoteContent) (note.SessionNote, error)
	// ShareNotes stamps sharedAt, making the shared fields visible to the
	// client. Idempotent and one-way: JustShared reports whether this call
	// performed the transition. Sharing without a note returns
	// note.ErrNoteNotFound.
	ShareNotes(ctx context.Context, practitionerID, bookingID string) (ShareResult, error)
}

// SessionNoteRepository is the outbound port for session-note persistence
// (the session_notes collection; bookingId is unique).
type SessionNoteRepository interface {
	// Create persists a new note, assigning its ID. A second note for the
	// same booking returns note.ErrNoteExists.
	Create(ctx context.Context, n note.SessionNote) (note.SessionNote, error)
	// FindByBookingID looks up the booking's single note; misses return
	// note.ErrNoteNotFound.
	FindByBookingID(ctx context.Context, bookingID string) (note.SessionNote, error)
	// Update persists a note's mutable state. Misses return
	// note.ErrNoteNotFound.
	Update(ctx context.Context, n note.SessionNote) (note.SessionNote, error)
}
