package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/agreement"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// AgreementRepository stores service agreements and the signatures given
// against them, in two collections.
//
// Signatures carry a unique index on (clientId, agreementId) — see
// indexes.go. That index is what makes signing idempotent rather than
// something the application has to serialise: a double-submitted booking
// form cannot produce two signatures for the same contract.
type AgreementRepository struct {
	agreements *mongo.Collection
	signatures *mongo.Collection
	executions *mongo.Collection
}

var _ ports.AgreementRepository = (*AgreementRepository)(nil)

func NewAgreementRepository(db *mongo.Database) *AgreementRepository {
	return &AgreementRepository{
		agreements: db.Collection("agreements"),
		signatures: db.Collection("agreement_signatures"),
		executions: db.Collection("agreement_executions"),
	}
}

type agreementDoc struct {
	ID                       bson.ObjectID `bson:"_id,omitempty"`
	PractitionerID           bson.ObjectID `bson:"practitionerId"`
	Key                      string        `bson:"key"`
	Title                    string        `bson:"title"`
	Body                     string        `bson:"body"`
	RequiresCountersignature bool          `bson:"requiresCountersignature"`
	CollectionID             string        `bson:"collectionId,omitempty"`
	Version                  int           `bson:"version"`
	Active                   bool          `bson:"active"`
	CreatedAt                bson.DateTime `bson:"createdAt"`
	UpdatedAt                bson.DateTime `bson:"updatedAt"`
}

type agreementSignatureDoc struct {
	EvidenceHash         string `bson:"evidenceHash,omitempty"`
	GuardianName         string `bson:"guardianName,omitempty"`
	ArchiveStatus        string `bson:"archiveStatus,omitempty"`
	Acknowledged         bool   `bson:"acknowledged,omitempty"`
	ContextID            string `bson:"contextId,omitempty"`
	SignerRole           string `bson:"signerRole,omitempty"`
	ParticipantName      string `bson:"participantName,omitempty"`
	GuardianRelationship string `bson:"guardianRelationship,omitempty"`
	GuardianEmail        string `bson:"guardianEmail,omitempty"`
	ActorID              string `bson:"actorId,omitempty"`
	ConsentBody          string `bson:"consentBody,omitempty"`
	ConsentVersion       string `bson:"consentVersion,omitempty"`

	StatementOfWork          *agreement.StatementOfWork `bson:"statementOfWork,omitempty"`
	SubmittedAt              *time.Time                 `bson:"submittedAt,omitempty"`
	ID                       bson.ObjectID              `bson:"_id,omitempty"`
	AgreementID              bson.ObjectID              `bson:"agreementId"`
	AgreementKey             string                     `bson:"agreementKey"`
	AgreementTitle           string                     `bson:"agreementTitle"`
	AgreementVersion         int                        `bson:"agreementVersion"`
	AgreementBody            string                     `bson:"agreementBody"`
	ClientID                 bson.ObjectID              `bson:"clientId"`
	ClientName               string                     `bson:"clientName"`
	ClientEmail              string                     `bson:"clientEmail"`
	SignedName               string                     `bson:"signedName"`
	RequiresCountersignature bool                       `bson:"requiresCountersignature"`
	PractitionerSignedName   string                     `bson:"practitionerSignedName,omitempty"`
	PractitionerSignedAt     *bson.DateTime             `bson:"practitionerSignedAt,omitempty"`
	SharedWithClient         bool                       `bson:"sharedWithClient"`
	BookingID                string                     `bson:"bookingId,omitempty"`
	SignedAt                 bson.DateTime              `bson:"signedAt"`
}

func newAgreementDoc(a agreement.Agreement) (agreementDoc, error) {
	pid, err := bson.ObjectIDFromHex(a.PractitionerID)
	if err != nil {
		return agreementDoc{}, fmt.Errorf("practitioner id: %w", err)
	}
	doc := agreementDoc{
		PractitionerID:           pid,
		Key:                      a.Key,
		Title:                    a.Title,
		Body:                     a.Body,
		RequiresCountersignature: a.RequiresCountersignature,
		CollectionID:             a.CollectionID,
		Version:                  a.Version,
		Active:                   a.Active,
		CreatedAt:                bson.NewDateTimeFromTime(a.CreatedAt),
		UpdatedAt:                bson.NewDateTimeFromTime(a.UpdatedAt),
	}
	if a.ID != "" {
		if oid, err := bson.ObjectIDFromHex(a.ID); err == nil {
			doc.ID = oid
		}
	}
	return doc, nil
}

func (d agreementDoc) toDomain() agreement.Agreement {
	return agreement.Agreement{
		ID:                       d.ID.Hex(),
		PractitionerID:           d.PractitionerID.Hex(),
		Key:                      d.Key,
		Title:                    d.Title,
		Body:                     d.Body,
		RequiresCountersignature: agreement.RequiresPractitionerSignature(d.Key),
		CollectionID:             d.CollectionID,
		Version:                  d.Version,
		Active:                   d.Active,
		CreatedAt:                d.CreatedAt.Time().UTC(),
		UpdatedAt:                d.UpdatedAt.Time().UTC(),
	}
}

func (d agreementSignatureDoc) toDomain() agreement.Signature {
	sig := agreement.Signature{
		EvidenceHash: d.EvidenceHash, GuardianName: d.GuardianName,
		ArchiveStatus:        d.ArchiveStatus,
		Acknowledged:         d.Acknowledged,
		ContextID:            d.ContextID,
		SignerRole:           d.SignerRole,
		ParticipantName:      d.ParticipantName,
		GuardianRelationship: d.GuardianRelationship,
		GuardianEmail:        d.GuardianEmail,
		ActorID:              d.ActorID,
		ConsentBody:          d.ConsentBody,
		ConsentVersion:       d.ConsentVersion,

		StatementOfWork: d.StatementOfWork, SubmittedAt: d.SubmittedAt,
		ID:                       d.ID.Hex(),
		AgreementID:              d.AgreementID.Hex(),
		AgreementKey:             d.AgreementKey,
		AgreementTitle:           d.AgreementTitle,
		AgreementVersion:         d.AgreementVersion,
		AgreementBody:            d.AgreementBody,
		ClientID:                 d.ClientID.Hex(),
		ClientName:               d.ClientName,
		ClientEmail:              d.ClientEmail,
		SignedName:               d.SignedName,
		RequiresCountersignature: agreement.RequiresPractitionerSignature(d.AgreementKey),
		PractitionerSignedName:   d.PractitionerSignedName,
		SharedWithClient:         d.SharedWithClient,
		BookingID:                d.BookingID,
		SignedAt:                 d.SignedAt.Time().UTC(),
	}
	if d.PractitionerSignedAt != nil {
		t := d.PractitionerSignedAt.Time().UTC()
		sig.PractitionerSignedAt = &t
	}
	return sig
}

func (r *AgreementRepository) List(ctx context.Context, practitionerID string, includeInactive bool) ([]agreement.Agreement, error) {
	pid, err := bson.ObjectIDFromHex(practitionerID)
	if err != nil {
		return []agreement.Agreement{}, nil
	}
	filter := bson.M{"practitionerId": pid}
	if !includeInactive {
		filter["active"] = true
	}
	cursor, err := r.agreements.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "title", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list agreements: %w", err)
	}
	var docs []agreementDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode agreements: %w", err)
	}
	out := make([]agreement.Agreement, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.toDomain())
	}
	return out, nil
}

func (r *AgreementRepository) GetByID(ctx context.Context, id string) (agreement.Agreement, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return agreement.Agreement{}, agreement.ErrAgreementNotFound
	}
	var doc agreementDoc
	if err := r.agreements.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return agreement.Agreement{}, agreement.ErrAgreementNotFound
		}
		return agreement.Agreement{}, fmt.Errorf("get agreement: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *AgreementRepository) GetByKey(ctx context.Context, practitionerID, key string) (agreement.Agreement, error) {
	pid, err := bson.ObjectIDFromHex(practitionerID)
	if err != nil {
		return agreement.Agreement{}, agreement.ErrAgreementNotFound
	}
	var doc agreementDoc
	err = r.agreements.FindOne(ctx, bson.M{"practitionerId": pid, "key": key}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return agreement.Agreement{}, agreement.ErrAgreementNotFound
		}
		return agreement.Agreement{}, fmt.Errorf("get agreement by key: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *AgreementRepository) Create(ctx context.Context, a agreement.Agreement) (agreement.Agreement, error) {
	doc, err := newAgreementDoc(a)
	if err != nil {
		return agreement.Agreement{}, err
	}
	doc.ID = bson.ObjectID{}
	res, err := r.agreements.InsertOne(ctx, doc)
	if err != nil {
		return agreement.Agreement{}, fmt.Errorf("insert agreement: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		a.ID = oid.Hex()
	}
	return a, nil
}

func (r *AgreementRepository) Update(ctx context.Context, a agreement.Agreement) (agreement.Agreement, error) {
	oid, err := bson.ObjectIDFromHex(a.ID)
	if err != nil {
		return agreement.Agreement{}, agreement.ErrAgreementNotFound
	}
	update := bson.M{"$set": bson.M{
		"title":                    a.Title,
		"body":                     a.Body,
		"requiresCountersignature": a.RequiresCountersignature,
		"collectionId":             a.CollectionID,
		"version":                  a.Version,
		"active":                   a.Active,
		"updatedAt":                bson.NewDateTimeFromTime(a.UpdatedAt),
	}}
	filter := bson.M{"_id": oid}
	if a.ExpectedVersion > 0 {
		filter["version"] = a.ExpectedVersion
	}
	res, err := updateOneWithEvent(ctx, r.agreements, filter, update)
	if err != nil {
		return agreement.Agreement{}, fmt.Errorf("update agreement: %w", err)
	}
	if res.MatchedCount == 0 {
		return agreement.Agreement{}, agreement.ErrAgreementChanged
	}
	return a, nil
}

// CreateSignature inserts the signature, or returns the one already on file
// when this client has signed this agreement before. Signing the same
// contract twice is a repeat of an action already taken, not an error to
// put in front of someone mid-booking.
func (r *AgreementRepository) CreateSignature(ctx context.Context, sig agreement.Signature) (agreement.Signature, error) {
	aid, err := bson.ObjectIDFromHex(sig.AgreementID)
	if err != nil {
		return agreement.Signature{}, agreement.ErrAgreementNotFound
	}
	cid, err := bson.ObjectIDFromHex(sig.ClientID)
	if err != nil {
		return agreement.Signature{}, agreement.ErrInvalidClient
	}
	doc := agreementSignatureDoc{
		EvidenceHash: sig.EvidenceHash, GuardianName: sig.GuardianName,
		ArchiveStatus:        "pending",
		Acknowledged:         sig.Acknowledged,
		ContextID:            sig.ContextID,
		SignerRole:           sig.SignerRole,
		ParticipantName:      sig.ParticipantName,
		GuardianRelationship: sig.GuardianRelationship,
		GuardianEmail:        sig.GuardianEmail,
		ActorID:              sig.ActorID,
		ConsentBody:          sig.ConsentBody,
		ConsentVersion:       sig.ConsentVersion,

		StatementOfWork: sig.StatementOfWork, SubmittedAt: sig.SubmittedAt,
		AgreementID:              aid,
		AgreementKey:             sig.AgreementKey,
		AgreementTitle:           sig.AgreementTitle,
		AgreementVersion:         sig.AgreementVersion,
		AgreementBody:            sig.AgreementBody,
		ClientID:                 cid,
		ClientName:               sig.ClientName,
		ClientEmail:              sig.ClientEmail,
		SignedName:               sig.SignedName,
		RequiresCountersignature: sig.RequiresCountersignature,
		PractitionerSignedName:   sig.PractitionerSignedName,
		SharedWithClient:         sig.SharedWithClient,
		BookingID:                sig.BookingID,
		SignedAt:                 bson.NewDateTimeFromTime(sig.SignedAt),
	}
	if sig.PractitionerSignedAt != nil {
		dt := bson.NewDateTimeFromTime(*sig.PractitionerSignedAt)
		doc.PractitionerSignedAt = &dt
	}
	coll := r.signatures
	if sig.ContextID != "" {
		coll = r.executions
	}
	res, err := insertOneWithEvent(ctx, coll, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			if sig.ContextID != "" {
				var existing agreementSignatureDoc
				err := coll.FindOne(ctx, bson.M{"clientId": cid, "agreementId": aid, "contextId": sig.ContextID, "agreementVersion": sig.AgreementVersion, "signerRole": sig.SignerRole}).Decode(&existing)
				return existing.toDomain(), err
			}
			return r.SignatureFor(ctx, sig.ClientID, sig.AgreementID)
		}
		return agreement.Signature{}, fmt.Errorf("insert signature: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		sig.ID = oid.Hex()
	}
	sig.ArchiveStatus = "pending"
	return sig, nil
}

func (r *AgreementRepository) UpdateSignature(ctx context.Context, sig agreement.Signature) (agreement.Signature, error) {
	oid, err := bson.ObjectIDFromHex(sig.ID)
	if err != nil {
		return agreement.Signature{}, agreement.ErrSignatureNotFound
	}
	setDoc := bson.M{
		"practitionerSignedName": sig.PractitionerSignedName,
		"sharedWithClient":       sig.SharedWithClient,
	}
	if sig.PractitionerSignedAt != nil {
		setDoc["practitionerSignedAt"] = bson.NewDateTimeFromTime(*sig.PractitionerSignedAt)
	}
	coll := r.signatures
	if sig.ContextID != "" {
		coll = r.executions
	}
	res, err := updateOneWithEvent(ctx, coll, bson.M{"_id": oid, "practitionerSignedAt": nil}, bson.M{"$set": setDoc})
	if err != nil {
		return agreement.Signature{}, fmt.Errorf("update signature: %w", err)
	}
	if res.MatchedCount == 0 {
		return agreement.Signature{}, agreement.ErrSignatureNotFound
	}
	return sig, nil
}

func (r *AgreementRepository) SignatureFor(ctx context.Context, clientID, agreementID string) (agreement.Signature, error) {
	cid, err := bson.ObjectIDFromHex(clientID)
	if err != nil {
		return agreement.Signature{}, agreement.ErrAgreementNotFound
	}
	aid, err := bson.ObjectIDFromHex(agreementID)
	if err != nil {
		return agreement.Signature{}, agreement.ErrAgreementNotFound
	}
	var doc agreementSignatureDoc
	err = r.signatures.FindOne(ctx, bson.M{"clientId": cid, "agreementId": aid}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return agreement.Signature{}, agreement.ErrAgreementNotFound
		}
		return agreement.Signature{}, fmt.Errorf("get signature: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *AgreementRepository) SignatureByID(ctx context.Context, id string) (agreement.Signature, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return agreement.Signature{}, agreement.ErrAgreementNotFound
	}
	for _, coll := range []*mongo.Collection{r.executions, r.signatures} {
		var doc agreementSignatureDoc
		err = coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc)
		if err == nil {
			return doc.toDomain(), nil
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return agreement.Signature{}, err
		}
	}
	return agreement.Signature{}, agreement.ErrAgreementNotFound
}

func (r *AgreementRepository) SignaturesForClient(ctx context.Context, clientID string) ([]agreement.Signature, error) {
	cid, err := bson.ObjectIDFromHex(clientID)
	if err != nil {
		return nil, agreement.ErrInvalidClient
	}
	out := []agreement.Signature{}
	for _, coll := range []*mongo.Collection{r.executions, r.signatures} {
		cursor, err := coll.Find(ctx, bson.M{"clientId": cid}, options.Find().SetSort(bson.D{{Key: "signedAt", Value: -1}}))
		if err != nil {
			return nil, err
		}
		var docs []agreementSignatureDoc
		if err = cursor.All(ctx, &docs); err != nil {
			return nil, err
		}
		for _, doc := range docs {
			out = append(out, doc.toDomain())
		}
	}
	return out, nil
}

// SetArchiveStatus only changes delivery metadata; the signed snapshot is never rewritten.
func (r *AgreementRepository) SetArchiveStatus(ctx context.Context, id, status string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	for _, coll := range []*mongo.Collection{r.executions, r.signatures} {
		result, err := coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{"archiveStatus": status}})
		if err != nil {
			return err
		}
		if result.MatchedCount > 0 {
			return nil
		}
	}
	return agreement.ErrSignatureNotFound
}
