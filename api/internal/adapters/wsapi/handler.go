package wsapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	domain "github.com/xcreativs/terios/api/internal/domain/signaling"
	"github.com/xcreativs/terios/api/internal/ports"
)

// Connection limits.
const (
	// maxMessageBytes bounds one signaling frame. An SDP offer with many
	// candidates is a few kilobytes; 64 KB is generous and stops a socket
	// being used to push megabytes through the server.
	maxMessageBytes = 64 * 1024
	// writeTimeout bounds one write to a peer.
	writeTimeout = 10 * time.Second
	// pingEvery is how often the server checks the connection is alive, so
	// a half-open socket is noticed instead of holding a place in the room.
	pingEvery = 30 * time.Second
)

// Handler serves the signaling WebSocket.
type Handler struct {
	svc ports.SignalingService
	hub *Hub
	// allowedOrigins is the exact set of sites permitted to open a socket.
	// Empty means same-origin only — never "any", because a WebSocket
	// handshake is not covered by the browser's same-origin policy and an
	// unchecked origin lets any page connect with the victim's ticket.
	allowedOrigins []string
	log            *slog.Logger
}

// NewHandler builds a signaling handler.
func NewHandler(svc ports.SignalingService, hub *Hub, allowedOrigins []string, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{svc: svc, hub: hub, allowedOrigins: allowedOrigins, log: log}
}

// ServeHTTP upgrades the request and runs the session.
//
// The ticket arrives as a query parameter because a browser cannot set
// headers on a WebSocket handshake. That is safe here only because the
// ticket is single-use and lives a minute: it is spent by this very
// request, so a copy in an access log is already worthless.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, bookingID string) {
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		writeHandshakeError(w, http.StatusUnauthorized, "a connection ticket is required")
		return
	}

	room, participant, err := h.svc.RedeemTicket(r.Context(), ticket, bookingID)
	if err != nil {
		h.writeRedeemError(w, err)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: h.allowedOrigins,
	})
	if err != nil {
		// Accept has already written its own response.
		h.log.Info("signaling handshake refused", "booking", bookingID, "error", err)
		return
	}
	conn.SetReadLimit(maxMessageBytes)

	h.run(r.Context(), conn, room, participant)
}

// run joins the room and pumps messages until the socket or the room's
// window closes.
func (h *Handler) run(ctx context.Context, conn *websocket.Conn, room domain.Room, participant domain.Participant) {
	// The connection lives no longer than the room's own closing time, so
	// a session cannot quietly continue past its window.
	ctx, cancel := context.WithDeadline(ctx, room.ClosesAt)
	defer cancel()

	outbound, others, err := h.hub.Join(room.BookingID, participant)
	if err != nil {
		h.closeWith(conn, websocket.StatusPolicyViolation, err)
		return
	}
	defer h.hub.Leave(room.BookingID, participant.PeerID)

	// The first frame tells the client who it is and who is already here,
	// so it knows whether to make the offer or wait for one.
	joined := Envelope{Type: domain.TypeJoined, From: participant.PeerID, Role: participant.Role}
	if len(others) > 0 {
		joined.Payload = peersPayload(others)
	}
	if err := writeEnvelope(ctx, conn, joined); err != nil {
		return
	}

	go h.writeLoop(ctx, conn, outbound)
	h.readLoop(ctx, conn, room, participant)
}

// readLoop reads from the socket until it closes.
func (h *Handler) readLoop(ctx context.Context, conn *websocket.Conn, room domain.Room, participant domain.Participant) {
	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			h.closeWith(conn, websocket.StatusUnsupportedData, domain.ErrInvalidMessage)
			return
		}

		var envelope Envelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			_ = writeEnvelope(ctx, conn, errorEnvelope("message must be JSON"))
			continue
		}
		if !envelope.Type.Valid() {
			// Refused, not relayed: the socket is for negotiating a call,
			// not a general channel between two accounts.
			_ = writeEnvelope(ctx, conn, errorEnvelope("unsupported message type"))
			continue
		}
		if envelope.Type == domain.TypePing {
			_ = writeEnvelope(ctx, conn, Envelope{Type: domain.TypePong})
			continue
		}
		if envelope.Type == domain.TypePong {
			continue
		}
		if err := h.hub.Relay(room.BookingID, participant, envelope); err != nil {
			_ = writeEnvelope(ctx, conn, errorEnvelope("that message cannot be relayed"))
		}
	}
}

// writeLoop pumps hub messages to the socket and keeps it alive.
func (h *Handler) writeLoop(ctx context.Context, conn *websocket.Conn, outbound <-chan Envelope) {
	ticker := time.NewTicker(pingEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// The room's window closed. Say so rather than dropping the
			// socket silently.
			h.closeWith(conn, websocket.StatusNormalClosure, domain.ErrRoomClosed)
			return
		case message, ok := <-outbound:
			if !ok {
				// The hub dropped this peer — a reconnection from the same
				// person took its place.
				_ = conn.Close(websocket.StatusNormalClosure, "replaced by a newer connection")
				return
			}
			if err := writeEnvelope(ctx, conn, message); err != nil {
				return
			}
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// writeEnvelope sends one message with a bounded write deadline.
func writeEnvelope(ctx context.Context, conn *websocket.Conn, envelope Envelope) error {
	raw, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, raw)
}

// errorEnvelope builds a client-visible error frame.
func errorEnvelope(reason string) Envelope {
	return Envelope{Type: domain.TypeError, Reason: reason}
}

// peersPayload describes who is already in the room.
func peersPayload(others []domain.Participant) json.RawMessage {
	type peerInfo struct {
		PeerID string      `json:"peerId"`
		Role   domain.Role `json:"role"`
	}
	infos := make([]peerInfo, 0, len(others))
	for _, other := range others {
		infos = append(infos, peerInfo{PeerID: other.PeerID, Role: other.Role})
	}
	raw, err := json.Marshal(map[string]any{"peers": infos})
	if err != nil {
		return nil
	}
	return raw
}

// closeWith closes the socket with a reason the client can show.
func (h *Handler) closeWith(conn *websocket.Conn, status websocket.StatusCode, cause error) {
	_ = conn.Close(status, cause.Error())
}

// writeRedeemError turns a failed ticket redemption into an HTTP response.
// The upgrade never happens, so the portal gets a status it can explain
// rather than a socket that closes for no stated reason.
func (h *Handler) writeRedeemError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrTicketInvalid):
		writeHandshakeError(w, http.StatusUnauthorized, "connection ticket is invalid or expired")
	case errors.Is(err, domain.ErrRoomNotOpenYet):
		writeHandshakeError(w, http.StatusForbidden, "the video room is not open yet")
	case errors.Is(err, domain.ErrRoomClosed):
		writeHandshakeError(w, http.StatusForbidden, "the video room for this session has closed")
	case errors.Is(err, domain.ErrSessionNotActive):
		writeHandshakeError(w, http.StatusForbidden, "this session is not active")
	default:
		writeHandshakeError(w, http.StatusNotFound, "session not found")
	}
}

// writeHandshakeError writes the API's standard error envelope. It is
// duplicated rather than imported from the HTTP adapter because this
// package must not depend on that one; the shape is part of the contract.
func writeHandshakeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": "signaling_unavailable", "message": message},
	})
}
