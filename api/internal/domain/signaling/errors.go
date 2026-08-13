package signaling

import "errors"

// Domain errors for the signaling slice.
var (
	// ErrSessionNotActive means the booking is cancelled, completed, or a
	// no-show — there is no session to join.
	ErrSessionNotActive = errors.New("this session is not active")
	// ErrRoomNotOpenYet means the join window has not started.
	ErrRoomNotOpenYet = errors.New("the video room is not open yet")
	// ErrRoomClosed means the join window has passed.
	ErrRoomClosed = errors.New("the video room for this session has closed")
	// ErrRoomFull means both places are taken. These are one-to-one
	// sessions; a third connection is refused rather than quietly turning
	// a private consultation into a group call.
	ErrRoomFull = errors.New("this session already has both participants")
	// ErrInvalidMessage means a message type outside the allowed set, or
	// one a participant is not permitted to send.
	ErrInvalidMessage = errors.New("invalid signaling message")
	// ErrTicketInvalid means the connection ticket is unknown, expired, or
	// already spent.
	ErrTicketInvalid = errors.New("connection ticket is invalid or expired")
)
