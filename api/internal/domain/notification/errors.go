package notification

import "errors"

// Domain errors for the notifications slice.
var (
	// ErrInvalidKind means a message kind outside the known set.
	ErrInvalidKind = errors.New("invalid notification kind")
	// ErrRecipientRequired guards against queueing a message with nowhere
	// to send it.
	ErrRecipientRequired = errors.New("notification recipient is required")
	// ErrInvalidTransition means the job's lifecycle forbids the move —
	// delivering a cancelled job, cancelling a sent one, and so on.
	ErrInvalidTransition = errors.New("invalid notification transition")
	// ErrJobNotFound means no job matches the lookup.
	ErrJobNotFound = errors.New("notification job not found")
	// ErrTemplateNotFound means no template is registered for a kind.
	ErrTemplateNotFound = errors.New("notification template not found")
)
