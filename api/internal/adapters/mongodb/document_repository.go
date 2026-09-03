package mongodb

import (
	"context"
	"errors"
	"fmt"

	"github.com/xcreativs/terios/api/internal/domain/document"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// DocumentRepository persists document records in the documents
// collection. clientId is stored as an ObjectID so this repository and the
// client-record counter read the same shape.
type DocumentRepository struct {
	coll *mongo.Collection
}

var _ ports.DocumentRepository = (*DocumentRepository)(nil)

// NewDocumentRepository binds the repository to documents.
func NewDocumentRepository(db *mongo.Database) *DocumentRepository {
	return &DocumentRepository{coll: db.Collection("documents")}
}

type documentDoc struct {
	ID              bson.ObjectID  `bson:"_id,omitempty"`
	Kind            string         `bson:"kind"`
	ClientID        *bson.ObjectID `bson:"clientId,omitempty"`
	UploadedBy      string         `bson:"uploadedBy"`
	PublicID        string         `bson:"publicId"`
	Filename        string         `bson:"filename"`
	Title           string         `bson:"title"`
	ResourceType    string         `bson:"resourceType"`
	Format          string         `bson:"format,omitempty"`
	Bytes           int64          `bson:"bytes"`
	VisibleToClient bool           `bson:"visibleToClient"`
	CreatedAt       bson.DateTime  `bson:"createdAt"`
	UpdatedAt       bson.DateTime  `bson:"updatedAt"`
}

func newDocumentDoc(d document.Document) (documentDoc, error) {
	doc := documentDoc{
		Kind:            string(d.Kind),
		UploadedBy:      d.UploadedBy,
		PublicID:        d.PublicID,
		Filename:        d.Filename,
		Title:           d.Title,
		ResourceType:    string(d.ResourceType),
		Format:          d.Format,
		Bytes:           d.Bytes,
		VisibleToClient: d.VisibleToClient,
		CreatedAt:       bson.NewDateTimeFromTime(d.CreatedAt),
		UpdatedAt:       bson.NewDateTimeFromTime(d.UpdatedAt),
	}
	if d.ClientID != "" {
		oid, err := bson.ObjectIDFromHex(d.ClientID)
		if err != nil {
			return documentDoc{}, document.ErrClientRequired
		}
		doc.ClientID = &oid
	}
	return doc, nil
}

func (d documentDoc) toDomain() document.Document {
	out := document.Document{
		ID:              d.ID.Hex(),
		Kind:            document.Kind(d.Kind),
		UploadedBy:      d.UploadedBy,
		PublicID:        d.PublicID,
		Filename:        d.Filename,
		Title:           d.Title,
		ResourceType:    document.ResourceType(d.ResourceType),
		Format:          d.Format,
		Bytes:           d.Bytes,
		VisibleToClient: d.VisibleToClient,
		CreatedAt:       d.CreatedAt.Time().UTC(),
		UpdatedAt:       d.UpdatedAt.Time().UTC(),
	}
	if d.ClientID != nil {
		out.ClientID = d.ClientID.Hex()
	}
	return out
}

func (r *DocumentRepository) Create(ctx context.Context, d document.Document) (document.Document, error) {
	doc, err := newDocumentDoc(d)
	if err != nil {
		return document.Document{}, err
	}
	res, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		return document.Document{}, fmt.Errorf("insert document: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		d.ID = oid.Hex()
	}
	return d, nil
}

// Update persists the editable fields only. The owner and the stored
// object are immutable here as well as in the domain: a repository that
// could re-point them would undo the guarantee.
func (r *DocumentRepository) Update(ctx context.Context, d document.Document) (document.Document, error) {
	oid, err := bson.ObjectIDFromHex(d.ID)
	if err != nil {
		return document.Document{}, document.ErrDocumentNotFound
	}
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{
		"title":           d.Title,
		"visibleToClient": d.VisibleToClient,
		"updatedAt":       bson.NewDateTimeFromTime(d.UpdatedAt),
	}})
	if err != nil {
		return document.Document{}, fmt.Errorf("update document: %w", err)
	}
	if res.MatchedCount == 0 {
		return document.Document{}, document.ErrDocumentNotFound
	}
	return d, nil
}

func (r *DocumentRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return document.ErrDocumentNotFound
	}
	if _, err := r.coll.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	return nil
}

func (r *DocumentRepository) FindByID(ctx context.Context, id string) (document.Document, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return document.Document{}, document.ErrDocumentNotFound
	}
	var doc documentDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return document.Document{}, document.ErrDocumentNotFound
		}
		return document.Document{}, fmt.Errorf("find document: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *DocumentRepository) ListByClient(ctx context.Context, clientID string) ([]document.Document, error) {
	oid, err := bson.ObjectIDFromHex(clientID)
	if err != nil {
		// An unusable id can own nothing; an empty list is the honest
		// answer, not an error about hex encoding.
		return []document.Document{}, nil
	}
	cursor, err := r.coll.Find(ctx, bson.M{"clientId": oid},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var docs []documentDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode documents: %w", err)
	}
	out := make([]document.Document, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.toDomain())
	}
	return out, nil
}

func (r *DocumentRepository) ListByKind(ctx context.Context, kind document.Kind) ([]document.Document, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"kind": string(kind)},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list documents by kind: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()
	var docs []documentDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode documents by kind: %w", err)
	}
	out := make([]document.Document, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.toDomain())
	}
	return out, nil
}
