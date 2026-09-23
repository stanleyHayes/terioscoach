package httpapi

import (
	"errors"
	"github.com/xcreativs/terios/api/internal/domain/booking"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/agreement"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// WithAgreements mounts the service-agreements slice (BE-14).
//
//	GET   /v1/services/{id}/agreement   → {agreement, signature} for the caller
//	POST  /v1/agreements/{id}/sign      → {signature}
//	GET   /v1/agreements/mine           → {items} the caller's signatures
//	GET   /v1/admin/agreements          → {items} incl. retired
//	POST  /v1/admin/agreements          → {agreement}
//	PATCH /v1/admin/agreements/{id}     → {agreement}
//	GET   /v1/admin/clients/{id}/agreements → {items}
//	POST  /v1/admin/signatures/{id}/countersign → {signature}
func WithAgreements(svc ports.AgreementService, auth ports.AuthService) Option {
	return func(s *Server) {
		if svc == nil {
			s.Router.HandleFunc("/v1/services/{id}/agreement", handleAgreementsUnavailable)
			s.Router.HandleFunc("/v1/agreements/{id}/sign", handleAgreementsUnavailable)
			return
		}
		h := &agreementHandler{svc: svc}

		s.Router.Group(func(r chi.Router) {
			r.Use(RequireAuth(auth))
			r.Get("/v1/services/{id}/agreement", h.forService)
			r.Get("/v1/services/{id}/guardian-consent", h.guardianForService)
			r.Get("/v1/guardian-consent", h.guardianSettings)
			r.Patch("/v1/guardian-consent", h.guardianSettings)
			r.Post("/v1/agreements/{id}/sign", h.sign)
			r.Post("/v1/agreements/{id}/submit", h.sign)
			r.Get("/v1/bookings/{id}/agreements", h.forBooking)
			r.Get("/v1/agreements/mine", h.mine)
			r.Get("/v1/agreements/signatures/{signatureId}/pdf", h.document)
		})

		s.Router.Group(func(r chi.Router) {
			r.Use(RequireAuth(auth), RequireRole(identity.RolePractitioner))
			r.Get("/v1/admin/agreements", h.list)
			r.Post("/v1/admin/agreements", h.create)
			r.Patch("/v1/admin/agreements/{id}", h.update)
			r.Get("/v1/admin/clients/{id}/agreements", h.forClient)
			r.Post("/v1/admin/signatures/{id}/countersign", h.countersign)
		})
	}
}

func handleAgreementsUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable",
		"service agreements are unavailable: database not connected")
}

type agreementHandler struct {
	svc ports.AgreementService
}

type agreementBody struct {
	ID                       string    `json:"id"`
	Key                      string    `json:"key"`
	Title                    string    `json:"title"`
	Body                     string    `json:"body"`
	RequiresCountersignature bool      `json:"requiresCountersignature"`
	CollectionID             string    `json:"collectionId,omitempty"`
	Version                  int       `json:"version"`
	Active                   bool      `json:"active"`
	Updated                  time.Time `json:"updatedAt"`
}

func newAgreementBody(a agreement.Agreement) agreementBody {
	return agreementBody{
		ID:                       a.ID,
		Key:                      a.Key,
		Title:                    a.Title,
		Body:                     a.Body,
		RequiresCountersignature: agreement.RequiresPractitionerSignature(a.Key),
		CollectionID:             a.CollectionID,
		Version:                  a.Version,
		Active:                   a.Active,
		Updated:                  a.UpdatedAt,
	}
}

type agreementSignatureBody struct {
	ArchiveStatus            string                     `json:"archiveStatus,omitempty"`
	SignerRole               string                     `json:"signerRole,omitempty"`
	ParticipantName          string                     `json:"participantName,omitempty"`
	ConsentVersion           string                     `json:"consentVersion,omitempty"`
	ContextID                string                     `json:"contextId,omitempty"`
	StatementOfWork          *agreement.StatementOfWork `json:"statementOfWork,omitempty"`
	SubmittedAt              *time.Time                 `json:"submittedAt,omitempty"`
	ID                       string                     `json:"id"`
	AgreementID              string                     `json:"agreementId"`
	AgreementTitle           string                     `json:"agreementTitle"`
	AgreementVersion         int                        `json:"agreementVersion"`
	ClientID                 string                     `json:"clientId"`
	ClientName               string                     `json:"clientName"`
	ClientEmail              string                     `json:"clientEmail"`
	SignedName               string                     `json:"signedName"`
	RequiresCountersignature bool                       `json:"requiresCountersignature"`
	PractitionerSignedName   string                     `json:"practitionerSignedName,omitempty"`
	PractitionerSignedAt     *time.Time                 `json:"practitionerSignedAt,omitempty"`
	Countersigned            bool                       `json:"countersigned"`
	BookingID                string                     `json:"bookingId,omitempty"`
	SignedAt                 time.Time                  `json:"signedAt"`
}

func newAgreementSignatureBody(s agreement.Signature) agreementSignatureBody {
	return agreementSignatureBody{
		ArchiveStatus: s.ArchiveStatus,
		SignerRole:    s.SignerRole, ParticipantName: s.ParticipantName, ConsentVersion: s.ConsentVersion, ContextID: s.ContextID,
		StatementOfWork: s.StatementOfWork, SubmittedAt: s.SubmittedAt,
		ID:                       s.ID,
		AgreementID:              s.AgreementID,
		AgreementTitle:           s.AgreementTitle,
		AgreementVersion:         s.AgreementVersion,
		ClientID:                 s.ClientID,
		ClientName:               s.ClientName,
		ClientEmail:              s.ClientEmail,
		SignedName:               s.SignedName,
		RequiresCountersignature: agreement.RequiresPractitionerSignature(s.AgreementKey),
		PractitionerSignedName:   s.PractitionerSignedName,
		PractitionerSignedAt:     s.PractitionerSignedAt,
		Countersigned:            s.PractitionerSignedAt != nil,
		BookingID:                s.BookingID,
		SignedAt:                 s.SignedAt,
	}
}

type agreementStatusItem struct {
	ConsentVersion       string                  `json:"consentVersion,omitempty"`
	Fee                  string                  `json:"fee,omitempty"`
	GuardianConsentReady bool                    `json:"guardianConsentReady"`
	Required             bool                    `json:"required"`
	Signed               bool                    `json:"signed"`
	Agreement            *agreementBody          `json:"agreement,omitempty"`
	Signature            *agreementSignatureBody `json:"signature,omitempty"`
}

// forService answers the booking wizard's question: is there an agreement
// in the way of this service, and has this client already signed it?
func (h *agreementHandler) forService(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	statuses, err := h.svc.StatusesForService(r.Context(), id.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if len(statuses) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"required":   false,
			"signed":     true,
			"agreements": []agreementStatusItem{},
		})
		return
	}

	allSigned := true
	var pendingStatus *ports.AgreementStatus
	items := make([]agreementStatusItem, 0, len(statuses))
	for i := range statuses {
		st := &statuses[i]
		signed := st.Signed()
		if !signed {
			allSigned = false
			if pendingStatus == nil {
				pendingStatus = st
			}
		}
		item := agreementStatusItem{
			Required: st.Agreement != nil,
			Signed:   signed,
		}
		if st.Agreement != nil {
			ab := newAgreementBody(*st.Agreement)
			item.Agreement = &ab
		}
		if st.Signature != nil {
			sb := newAgreementSignatureBody(*st.Signature)
			item.Signature = &sb
		}
		items = append(items, item)
	}

	activeStatus := pendingStatus
	if activeStatus == nil {
		activeStatus = &statuses[0]
	}

	out := map[string]any{
		"required":   true,
		"signed":     allSigned,
		"agreements": items,
	}
	if activeStatus.Agreement != nil {
		out["agreement"] = newAgreementBody(*activeStatus.Agreement)
	}
	if activeStatus.Signature != nil {
		out["signature"] = newAgreementSignatureBody(*activeStatus.Signature)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *agreementHandler) countersign(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	var req struct {
		SignedName string `json:"signedName"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	sig, err := h.svc.Countersign(r.Context(), id, chi.URLParam(r, "id"), req.SignedName)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]agreementSignatureBody{"signature": newAgreementSignatureBody(sig)})
}

// sign records the caller's acceptance. The caller is always the signatory:
// a client id is never taken from the body, so no one can sign for someone
// else.
func (h *agreementHandler) sign(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	var req struct {
		ConsentVersion   string                     `json:"consentVersion"`
		Acknowledged     bool                       `json:"acknowledged"`
		SignerRole       string                     `json:"signerRole"`
		AgreementVersion int                        `json:"agreementVersion"`
		SignedName       string                     `json:"signedName"`
		BookingID        string                     `json:"bookingId"`
		StatementOfWork  *agreement.StatementOfWork `json:"statementOfWork"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	sig, err := h.svc.Sign(r.Context(), ports.SignRequest{
		ConsentVersion: req.ConsentVersion,
		Acknowledged:   req.Acknowledged, SignerRole: req.SignerRole, AgreementVersion: req.AgreementVersion,
		StatementOfWork: req.StatementOfWork,
		AgreementID:     chi.URLParam(r, "id"),
		ClientID:        id.UserID,
		SignedName:      req.SignedName,
		BookingID:       req.BookingID,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]agreementSignatureBody{"signature": newAgreementSignatureBody(sig)})
}

// document streams the signed agreement as a PDF. One route serves both
// sides — the service decides whether this caller is entitled to this
// signature — so there is no second, differently-guarded copy of the rule.
func (h *agreementHandler) document(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	doc, err := h.svc.SignedDocument(r.Context(), id, chi.URLParam(r, "signatureId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+doc.Filename+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(doc.Data)
}

func (h *agreementHandler) mine(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	h.writeSignatures(w, r, id.UserID)
}

func (h *agreementHandler) forClient(w http.ResponseWriter, r *http.Request) {
	h.writeSignatures(w, r, chi.URLParam(r, "id"))
}

func (h *agreementHandler) writeSignatures(w http.ResponseWriter, r *http.Request, clientID string) {
	items, err := h.svc.SignaturesForClient(r.Context(), clientID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]agreementSignatureBody, 0, len(items))
	for _, sig := range items {
		if caller, ok := IdentityFromContext(r.Context()); ok && caller.Role == identity.RolePractitioner {
			owner, err := h.svc.Get(r.Context(), sig.AgreementID)
			if err != nil || owner.PractitionerID != caller.UserID {
				continue
			}
		}
		out = append(out, newAgreementSignatureBody(sig))
	}
	writeJSON(w, http.StatusOK, map[string][]agreementSignatureBody{"items": out})
}

func (h *agreementHandler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	items, err := h.svc.List(r.Context(), id.UserID, true)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]agreementBody, 0, len(items))
	for _, a := range items {
		out = append(out, newAgreementBody(a))
	}
	writeJSON(w, http.StatusOK, map[string][]agreementBody{"items": out})
}

func (h *agreementHandler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	var req struct {
		Key   string `json:"key"`
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	a, err := h.svc.Create(r.Context(), id.UserID, ports.AgreementDraft{
		Key:   req.Key,
		Title: req.Title,
		Body:  req.Body,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]agreementBody{"agreement": newAgreementBody(a)})
}

func (h *agreementHandler) update(w http.ResponseWriter, r *http.Request) {
	caller, ok := identityOr401(w, r)
	if !ok {
		return
	}
	owner, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil || owner.PractitionerID != caller.UserID {
		writeDomainError(w, agreement.ErrAgreementNotFound)
		return
	}
	var req struct {
		Title  *string `json:"title"`
		Body   *string `json:"body"`
		Active *bool   `json:"active"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	a, err := h.svc.Update(r.Context(), chi.URLParam(r, "id"), ports.AgreementPatch{
		Title:  req.Title,
		Body:   req.Body,
		Active: req.Active,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]agreementBody{"agreement": newAgreementBody(a)})
}

func (h *agreementHandler) forBooking(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	statuses, err := h.svc.StatusesForBooking(r.Context(), id, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]agreementStatusItem, 0, len(statuses))
	for _, st := range statuses {
		item := agreementStatusItem{Required: st.Agreement != nil, Signed: st.Signed(), Fee: st.Fee, GuardianConsentReady: st.GuardianConsentReady, ConsentVersion: st.ConsentVersion}
		if st.Agreement != nil {
			a := newAgreementBody(*st.Agreement)
			item.Agreement = &a
		}
		if st.Signature != nil {
			sig := newAgreementSignatureBody(*st.Signature)
			item.Signature = &sig
		}
		items = append(items, item)
	}
	ready, message := false, "Session readiness unavailable"
	if gate, ok := h.svc.(ports.BookingReadiness); ok {
		if err := gate.RequireBookingReady(r.Context(), chi.URLParam(r, "id")); err != nil {
			message = "Session requirements could not be checked. Try again."
			if errors.Is(err, agreement.ErrAgreementRequired) || errors.Is(err, booking.ErrInvalidTransition) {
				message = err.Error()
			}
		} else {
			ready = true
			message = "All session requirements are complete"
		}
	}
	writeJSON(w, 200, map[string]any{"items": items, "ready": ready, "readinessMessage": message})
}

func (h *agreementHandler) guardianForService(w http.ResponseWriter, r *http.Request) {
	a, err := h.svc.GuardianConsentForService(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, 200, map[string]agreementBody{"agreement": newAgreementBody(a)})
}
func (h *agreementHandler) guardianSettings(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	if id.Role != identity.RolePractitioner {
		writeError(w, 403, "forbidden", "practitioner access required")
		return
	}
	if r.Method == http.MethodPatch {
		var req struct {
			Body string `json:"body"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		a, err := h.svc.UpdateGuardianConsent(r.Context(), id.UserID, req.Body)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, 200, map[string]agreementBody{"agreement": newAgreementBody(a)})
		return
	}
	a, err := h.svc.GuardianConsent(r.Context(), id.UserID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, 200, map[string]agreementBody{"agreement": newAgreementBody(a)})
}
