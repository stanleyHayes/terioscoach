package mongodb

import (
	"context"
	"errors"
	"fmt"

	"github.com/xcreativs/terios/api/internal/domain/form"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// FormRepository persists form definitions in the forms collection.
type FormRepository struct {
	coll *mongo.Collection
}

var _ ports.FormRepository = (*FormRepository)(nil)

// NewFormRepository binds the repository to forms.
func NewFormRepository(db *mongo.Database) *FormRepository {
	return &FormRepository{coll: db.Collection("forms")}
}

type fieldDoc struct {
	Key      string   `bson:"key"`
	Label    string   `bson:"label"`
	Type     string   `bson:"type"`
	Required bool     `bson:"required"`
	HelpText string   `bson:"helpText,omitempty"`
	Options  []string `bson:"options"`
}

type formDoc struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	Title       string        `bson:"title"`
	Description string        `bson:"description,omitempty"`
	Fields      []fieldDoc    `bson:"fields"`
	Template    bool          `bson:"template"`
	SortOrder   int           `bson:"sortOrder"`
	Active      bool          `bson:"active"`
	CreatedAt   bson.DateTime `bson:"createdAt"`
	UpdatedAt   bson.DateTime `bson:"updatedAt"`
}

func newFormDoc(f form.Form) formDoc {
	fields := make([]fieldDoc, 0, len(f.Fields))
	for _, field := range f.Fields {
		options := field.Options
		if options == nil {
			options = []string{}
		}
		fields = append(fields, fieldDoc{
			Key:      field.Key,
			Label:    field.Label,
			Type:     string(field.Type),
			Required: field.Required,
			HelpText: field.HelpText,
			Options:  options,
		})
	}
	return formDoc{
		Title:       f.Title,
		Description: f.Description,
		Fields:      fields,
		Template:    f.Template,
		SortOrder:   f.SortOrder,
		Active:      f.Active,
		CreatedAt:   bson.NewDateTimeFromTime(f.CreatedAt),
		UpdatedAt:   bson.NewDateTimeFromTime(f.UpdatedAt),
	}
}

func (d formDoc) toDomain() form.Form {
	fields := make([]form.Field, 0, len(d.Fields))
	for _, field := range d.Fields {
		options := field.Options
		if options == nil {
			options = []string{}
		}
		fields = append(fields, form.Field{
			Key:      field.Key,
			Label:    field.Label,
			Type:     form.FieldType(field.Type),
			Required: field.Required,
			HelpText: field.HelpText,
			Options:  options,
		})
	}
	return form.Form{
		ID:          d.ID.Hex(),
		Title:       d.Title,
		Description: d.Description,
		Fields:      fields,
		Template:    d.Template,
		SortOrder:   d.SortOrder,
		Active:      d.Active,
		CreatedAt:   d.CreatedAt.Time().UTC(),
		UpdatedAt:   d.UpdatedAt.Time().UTC(),
	}
}

func (r *FormRepository) Create(ctx context.Context, f form.Form) (form.Form, error) {
	res, err := r.coll.InsertOne(ctx, newFormDoc(f))
	if err != nil {
		return form.Form{}, fmt.Errorf("insert form: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		f.ID = oid.Hex()
	}
	return f, nil
}

func (r *FormRepository) Update(ctx context.Context, f form.Form) (form.Form, error) {
	oid, err := bson.ObjectIDFromHex(f.ID)
	if err != nil {
		return form.Form{}, form.ErrFormNotFound
	}
	doc := newFormDoc(f)
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{
		"title":       doc.Title,
		"description": doc.Description,
		"fields":      doc.Fields,
		"template":    doc.Template,
		"sortOrder":   doc.SortOrder,
		"active":      doc.Active,
		"updatedAt":   doc.UpdatedAt,
	}})
	if err != nil {
		return form.Form{}, fmt.Errorf("update form: %w", err)
	}
	if res.MatchedCount == 0 {
		return form.Form{}, form.ErrFormNotFound
	}
	return f, nil
}

func (r *FormRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return form.ErrFormNotFound
	}
	if _, err := r.coll.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return fmt.Errorf("delete form: %w", err)
	}
	return nil
}

func (r *FormRepository) FindByID(ctx context.Context, id string) (form.Form, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return form.Form{}, form.ErrFormNotFound
	}
	var doc formDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return form.Form{}, form.ErrFormNotFound
		}
		return form.Form{}, fmt.Errorf("find form: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *FormRepository) List(ctx context.Context, activeOnly bool) ([]form.Form, error) {
	query := bson.M{}
	if activeOnly {
		query["active"] = true
	}
	sort := bson.D{{Key: "sortOrder", Value: 1}, {Key: "title", Value: 1}}
	cursor, err := r.coll.Find(ctx, query, options.Find().SetSort(sort))
	if err != nil {
		return nil, fmt.Errorf("list forms: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var docs []formDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode forms: %w", err)
	}
	out := make([]form.Form, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.toDomain())
	}
	return out, nil
}

// FormSubmissionRepository persists filled-in forms in form_submissions.
// clientId is stored as an ObjectID so the client-record counter and this
// repository read the same shape.
type FormSubmissionRepository struct {
	coll *mongo.Collection
}

var _ ports.FormSubmissionRepository = (*FormSubmissionRepository)(nil)

// NewFormSubmissionRepository binds the repository to form_submissions.
func NewFormSubmissionRepository(db *mongo.Database) *FormSubmissionRepository {
	return &FormSubmissionRepository{coll: db.Collection("form_submissions")}
}

type answerDoc struct {
	Value  string   `bson:"value,omitempty"`
	Values []string `bson:"values,omitempty"`
}

type signatureDoc struct {
	TypedName string        `bson:"typedName"`
	ImageData string        `bson:"imageData"`
	SignedAt  bson.DateTime `bson:"signedAt"`
	SignedIP  string        `bson:"signedIp,omitempty"`
	Hash      string        `bson:"hash"`
}

type submissionDoc struct {
	ID          bson.ObjectID        `bson:"_id,omitempty"`
	FormID      bson.ObjectID        `bson:"formId"`
	FormTitle   string               `bson:"formTitle"`
	ClientID    bson.ObjectID        `bson:"clientId"`
	BookingID   string               `bson:"bookingId,omitempty"`
	Status      string               `bson:"status"`
	Answers     map[string]answerDoc `bson:"answers"`
	Signature   *signatureDoc        `bson:"signature,omitempty"`
	AssignedAt  bson.DateTime        `bson:"assignedAt"`
	SubmittedAt *bson.DateTime       `bson:"submittedAt,omitempty"`
	CreatedAt   bson.DateTime        `bson:"createdAt"`
	UpdatedAt   bson.DateTime        `bson:"updatedAt"`
}

func newSubmissionDoc(s form.Submission) (submissionDoc, error) {
	formID, err := bson.ObjectIDFromHex(s.FormID)
	if err != nil {
		return submissionDoc{}, form.ErrFormNotFound
	}
	clientID, err := bson.ObjectIDFromHex(s.ClientID)
	if err != nil {
		return submissionDoc{}, form.ErrInvalidClient
	}

	answers := make(map[string]answerDoc, len(s.Answers))
	for key, answer := range s.Answers {
		answers[key] = answerDoc{Value: answer.Value, Values: answer.Values}
	}

	doc := submissionDoc{
		FormID:     formID,
		FormTitle:  s.FormTitle,
		ClientID:   clientID,
		BookingID:  s.BookingID,
		Status:     string(s.Status),
		Answers:    answers,
		AssignedAt: bson.NewDateTimeFromTime(s.AssignedAt),
		CreatedAt:  bson.NewDateTimeFromTime(s.CreatedAt),
		UpdatedAt:  bson.NewDateTimeFromTime(s.UpdatedAt),
	}
	if s.SubmittedAt != nil {
		stamp := bson.NewDateTimeFromTime(*s.SubmittedAt)
		doc.SubmittedAt = &stamp
	}
	if s.Signature != nil {
		doc.Signature = &signatureDoc{
			TypedName: s.Signature.TypedName,
			ImageData: s.Signature.ImageData,
			SignedAt:  bson.NewDateTimeFromTime(s.Signature.SignedAt),
			SignedIP:  s.Signature.SignedIP,
			Hash:      s.Signature.Hash,
		}
	}
	return doc, nil
}

func (d submissionDoc) toDomain() form.Submission {
	answers := make(map[string]form.Answer, len(d.Answers))
	for key, answer := range d.Answers {
		answers[key] = form.Answer{Value: answer.Value, Values: answer.Values}
	}
	s := form.Submission{
		ID:         d.ID.Hex(),
		FormID:     d.FormID.Hex(),
		FormTitle:  d.FormTitle,
		ClientID:   d.ClientID.Hex(),
		BookingID:  d.BookingID,
		Status:     form.SubmissionStatus(d.Status),
		Answers:    answers,
		AssignedAt: d.AssignedAt.Time().UTC(),
		CreatedAt:  d.CreatedAt.Time().UTC(),
		UpdatedAt:  d.UpdatedAt.Time().UTC(),
	}
	if d.SubmittedAt != nil {
		at := d.SubmittedAt.Time().UTC()
		s.SubmittedAt = &at
	}
	if d.Signature != nil {
		s.Signature = &form.Signature{
			TypedName: d.Signature.TypedName,
			ImageData: d.Signature.ImageData,
			SignedAt:  d.Signature.SignedAt.Time().UTC(),
			SignedIP:  d.Signature.SignedIP,
			Hash:      d.Signature.Hash,
		}
	}
	return s
}

func (r *FormSubmissionRepository) Create(ctx context.Context, s form.Submission) (form.Submission, error) {
	doc, err := newSubmissionDoc(s)
	if err != nil {
		return form.Submission{}, err
	}
	res, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		return form.Submission{}, fmt.Errorf("insert form submission: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		s.ID = oid.Hex()
	}
	return s, nil
}

func (r *FormSubmissionRepository) Update(ctx context.Context, s form.Submission) (form.Submission, error) {
	oid, err := bson.ObjectIDFromHex(s.ID)
	if err != nil {
		return form.Submission{}, form.ErrSubmissionNotFound
	}
	doc, err := newSubmissionDoc(s)
	if err != nil {
		return form.Submission{}, err
	}
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{
		"status":      doc.Status,
		"answers":     doc.Answers,
		"signature":   doc.Signature,
		"submittedAt": doc.SubmittedAt,
		"updatedAt":   doc.UpdatedAt,
	}})
	if err != nil {
		return form.Submission{}, fmt.Errorf("update form submission: %w", err)
	}
	if res.MatchedCount == 0 {
		return form.Submission{}, form.ErrSubmissionNotFound
	}
	return s, nil
}

func (r *FormSubmissionRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return form.ErrSubmissionNotFound
	}
	if _, err := r.coll.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return fmt.Errorf("delete form submission: %w", err)
	}
	return nil
}

func (r *FormSubmissionRepository) FindByID(ctx context.Context, id string) (form.Submission, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return form.Submission{}, form.ErrSubmissionNotFound
	}
	var doc submissionDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return form.Submission{}, form.ErrSubmissionNotFound
		}
		return form.Submission{}, fmt.Errorf("find form submission: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *FormSubmissionRepository) List(ctx context.Context, filter ports.SubmissionFilter) ([]form.Submission, error) {
	query, ok := submissionQuery(filter)
	if !ok {
		// An unusable id can match nothing; an empty result is the honest
		// answer, not an error about hex encoding.
		return []form.Submission{}, nil
	}
	cursor, err := r.coll.Find(ctx, query, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list form submissions: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var docs []submissionDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode form submissions: %w", err)
	}
	out := make([]form.Submission, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.toDomain())
	}
	return out, nil
}

func (r *FormSubmissionRepository) HasOpenAssignment(ctx context.Context, clientID, formID string) (bool, error) {
	query, ok := submissionQuery(ports.SubmissionFilter{
		ClientID: clientID,
		FormID:   formID,
		Status:   form.StatusAssigned,
	})
	if !ok {
		return false, nil
	}
	n, err := r.coll.CountDocuments(ctx, query)
	if err != nil {
		return false, fmt.Errorf("count open assignments: %w", err)
	}
	return n > 0, nil
}

// submissionQuery builds the filter, reporting false when an id cannot be
// an ObjectID and therefore cannot match anything.
func submissionQuery(filter ports.SubmissionFilter) (bson.M, bool) {
	query := bson.M{}
	if filter.ClientID != "" {
		oid, err := bson.ObjectIDFromHex(filter.ClientID)
		if err != nil {
			return nil, false
		}
		query["clientId"] = oid
	}
	if filter.FormID != "" {
		oid, err := bson.ObjectIDFromHex(filter.FormID)
		if err != nil {
			return nil, false
		}
		query["formId"] = oid
	}
	if filter.BookingID != "" {
		query["bookingId"] = filter.BookingID
	}
	if filter.Status != "" {
		query["status"] = string(filter.Status)
	}
	return query, true
}
