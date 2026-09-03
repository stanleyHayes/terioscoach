package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/document"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// WithDocuments mounts the document routes backed by the document port
// (BE-11). /v1/admin/documents is the practitioner's file management;
// /v1/documents is the client's own library. A nil service — no database
// or no media credentials — keeps the routes mounted but answering 503.
func WithDocuments(svc ports.DocumentService, auth ports.AuthService) Option {
	return func(s *Server) {
		s.Router.Route("/v1/documents", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleDocumentsUnavailable)
				r.HandleFunc("/", handleDocumentsUnavailable)
				return
			}
			h := &documentHandler{svc: svc}
			r.Use(RequireAuth(auth), RequireRole(identity.RoleClient))
			r.Get("/mine", h.listMine)
			r.Get("/mine/{id}/url", h.downloadMine)
		})

		s.Router.Route("/v1/admin/documents", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleDocumentsUnavailable)
				r.HandleFunc("/", handleDocumentsUnavailable)
				return
			}
			h := &documentHandler{svc: svc}
			r.Use(RequireAuth(auth), RequireRole(identity.RolePractitioner))
			r.Post("/sign-upload", h.signUpload)
			r.Post("/", h.record)
			r.Get("/", h.list)
			r.Get("/media", h.listMedia)
			r.Get("/{id}/url", h.downloadAny)
			r.Patch("/{id}", h.patch)
			r.Delete("/{id}", h.delete)
		})
	}
}

func (h *documentHandler) listMedia(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListCMSImages(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	type mediaBody struct {
		ID        string    `json:"id"`
		URL       string    `json:"url"`
		Title     string    `json:"title"`
		Filename  string    `json:"filename"`
		Bytes     int64     `json:"bytes"`
		CreatedAt time.Time `json:"createdAt"`
	}
	out := make([]mediaBody, 0, len(items))
	for _, item := range items {
		out = append(out, mediaBody{item.ID, item.URL, item.Title, item.Filename, item.Bytes, item.CreatedAt.UTC()})
	}
	writeJSON(w, http.StatusOK, map[string][]mediaBody{"items": out})
}

// handleDocumentsUnavailable answers every document route when the
// database or the media store is not configured.
func handleDocumentsUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "documents are unavailable: storage not configured")
}

type documentHandler struct {
	svc ports.DocumentService
}

// documentBody is the record as both apps see it.
//
// publicId is deliberately absent. It is the media store's own handle, and
// a client who learned one could try it against the store directly; every
// download goes through this API so ownership is checked first.
type documentBody struct {
	ID              string    `json:"id"`
	Kind            string    `json:"kind"`
	ClientID        string    `json:"clientId,omitempty"`
	Title           string    `json:"title"`
	Filename        string    `json:"filename"`
	Format          string    `json:"format,omitempty"`
	Bytes           int64     `json:"bytes"`
	VisibleToClient bool      `json:"visibleToClient"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func newDocumentBody(d document.Document) documentBody {
	return documentBody{
		ID:              d.ID,
		Kind:            string(d.Kind),
		ClientID:        d.ClientID,
		Title:           d.Title,
		Filename:        d.Filename,
		Format:          d.Format,
		Bytes:           d.Bytes,
		VisibleToClient: d.VisibleToClient,
		CreatedAt:       d.CreatedAt.UTC(),
		UpdatedAt:       d.UpdatedAt.UTC(),
	}
}

// clientDocumentBody is the client's view. It omits visibleToClient (every
// document they can see is shared, by construction) and the practice-side
// ids.
type clientDocumentBody struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Filename  string    `json:"filename"`
	Format    string    `json:"format,omitempty"`
	Bytes     int64     `json:"bytes"`
	CreatedAt time.Time `json:"createdAt"`
}

func newClientDocumentBody(d document.Document) clientDocumentBody {
	return clientDocumentBody{
		ID:        d.ID,
		Title:     d.Title,
		Filename:  d.Filename,
		Format:    d.Format,
		Bytes:     d.Bytes,
		CreatedAt: d.CreatedAt.UTC(),
	}
}

// ---- Practitioner ----

func (h *documentHandler) signUpload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind     string `json:"kind"`
		ClientID string `json:"clientId"`
		Filename string `json:"filename"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	signed, err := h.svc.SignUpload(r.Context(), ports.UploadRequest{
		Kind:     document.Kind(req.Kind),
		ClientID: req.ClientID,
		Filename: req.Filename,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":       signed.URL,
		"fields":    signed.Fields,
		"expiresAt": signed.ExpiresAt.UTC(),
	})
}

func (h *documentHandler) record(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	var req struct {
		Kind     string `json:"kind"`
		ClientID string `json:"clientId"`
		PublicID string `json:"publicId"`
		Filename string `json:"filename"`
		Bytes    int64  `json:"bytes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	d, err := h.svc.RecordUpload(r.Context(), id.UserID, ports.RecordUploadInput{
		Kind:     document.Kind(req.Kind),
		ClientID: req.ClientID,
		PublicID: req.PublicID,
		Filename: req.Filename,
		Bytes:    req.Bytes,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]documentBody{"document": newDocumentBody(d)})
}

func (h *documentHandler) list(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("clientId")
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "clientId is required")
		return
	}
	items, err := h.svc.ListForClient(r.Context(), clientID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]documentBody, 0, len(items))
	for _, d := range items {
		out = append(out, newDocumentBody(d))
	}
	writeJSON(w, http.StatusOK, map[string][]documentBody{"items": out})
}

func (h *documentHandler) patch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title           *string `json:"title"`
		VisibleToClient *bool   `json:"visibleToClient"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	d, err := h.svc.UpdateDocument(r.Context(), chi.URLParam(r, "id"), document.Patch{
		Title:           req.Title,
		VisibleToClient: req.VisibleToClient,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]documentBody{"document": newDocumentBody(d)})
}

func (h *documentHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteDocument(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *documentHandler) downloadAny(w http.ResponseWriter, r *http.Request) {
	url, err := h.svc.DownloadURLForPractitioner(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeDownloadURL(w, url)
}

// ---- Client ----

func (h *documentHandler) listMine(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListMine(r.Context(), id.UserID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]clientDocumentBody, 0, len(items))
	for _, d := range items {
		out = append(out, newClientDocumentBody(d))
	}
	writeJSON(w, http.StatusOK, map[string][]clientDocumentBody{"items": out})
}

func (h *documentHandler) downloadMine(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	url, err := h.svc.DownloadURLForClient(r.Context(), id.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeDownloadURL(w, url)
}

// writeDownloadURL returns the signed link as JSON rather than redirecting.
//
// A 302 would put the signed URL in the browser's history and in any
// referrer that follows; handing it back as a value lets the app fetch it
// once and drop it. The expiry is stated so a UI can refresh rather than
// letting a stale link fail in a person's hands.
func writeDownloadURL(w http.ResponseWriter, url string) {
	writeJSON(w, http.StatusOK, map[string]any{
		"url":       url,
		"expiresIn": int(ports.DefaultDeliveryTTL.Seconds()),
	})
}
