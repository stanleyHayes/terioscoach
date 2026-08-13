package integration

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/xcreativs/terios/api/internal/adapters/mongodb"
	domainbooking "github.com/xcreativs/terios/api/internal/domain/booking"
)

// ---------------------------------------------------------------------
// Resend (BE-09 / FND-06)
//
// The failure this guards against is the quiet one. An unverified sending
// domain does not error — Resend accepts the message and does not deliver
// it. Every test in this repo would stay green while no client ever
// received a confirmation.
// ---------------------------------------------------------------------

func TestLiveResendKeyAndSendingDomain(t *testing.T) {
	env := requireIntegration(t)
	apiKey := env["RESEND_API_KEY"]
	if placeholder(apiKey) {
		t.Skip("RESEND_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.resend.com/domains", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("reach Resend: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if res.StatusCode == http.StatusUnauthorized {
		t.Fatal("Resend rejected the API key")
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("Resend answered %d: %s", res.StatusCode, body)
	}

	var listed struct {
		Data []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Region string `json:"region"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatalf("decode domains: %v", err)
	}

	// The address the API actually sends from, e.g.
	// "Terios Wellness Spa <no-reply@terioswellness.com>".
	from := env["RESEND_FROM"]
	sendingDomain := from
	if i := strings.LastIndex(from, "@"); i >= 0 {
		sendingDomain = strings.TrimSuffix(from[i+1:], ">")
	}

	if len(listed.Data) == 0 {
		t.Errorf("no domains are registered with Resend, so mail from %q will be accepted and never delivered",
			sendingDomain)
		return
	}

	var found, verified bool
	for _, domain := range listed.Data {
		t.Logf("domain %s: %s (%s)", domain.Name, domain.Status, domain.Region)
		if strings.EqualFold(domain.Name, sendingDomain) {
			found = true
			verified = domain.Status == "verified"
		}
	}

	if !found {
		t.Errorf("RESEND_FROM sends as %q, which is not a domain registered with this account — "+
			"mail will be accepted and silently dropped", sendingDomain)
	} else if !verified {
		t.Errorf("%q is registered but NOT verified — publish the DKIM/SPF records at the registrar, "+
			"or every confirmation and reminder is accepted and never delivered", sendingDomain)
	}
}

// ---------------------------------------------------------------------
// MongoDB Atlas (FND-05)
//
// The largest untested surface in the project. Every repository is written
// against a driver whose bson tags, index options and error shapes are
// assumptions until a real server sees them — and the whole booking
// correctness argument rests on a partial unique index that no fake can
// prove exists.
// ---------------------------------------------------------------------

// mongoDB dials Atlas and returns a handle plus a cleanup.
func mongoDB(t *testing.T) (*mongodb.Client, string) {
	t.Helper()
	env := requireIntegration(t)
	uri := env["MONGODB_URI"]
	if placeholder(uri) {
		t.Skip("MONGODB_URI still contains a placeholder — replace <db_username> with the Atlas database user")
	}

	name := env["MONGODB_DB"]
	if name == "" || name == "terios" {
		// Never against the production database. An integration test that
		// writes bookings into the live collection is a worse problem than
		// the one it was written to find.
		name = "terios_integration"
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	client, err := mongodb.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("connect to Atlas: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Disconnect(context.Background())
	})
	return client, name
}

func TestLiveMongoConnects(t *testing.T) {
	client, name := mongoDB(t)

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	t.Logf("connected, using database %q", name)
}

func TestLiveMongoIndexesApply(t *testing.T) {
	client, name := mongoDB(t)
	db := client.Database(name)

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	// Idempotent by design — the API runs this on every boot.
	if err := mongodb.EnsureIndexes(ctx, db); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}
	if err := mongodb.EnsureIndexes(ctx, db); err != nil {
		t.Fatalf("EnsureIndexes is not idempotent, so every restart would fail: %v", err)
	}

	cursor, err := db.Collection("bookings").Indexes().List(ctx)
	if err != nil {
		t.Fatalf("list booking indexes: %v", err)
	}
	var indexes []map[string]any
	if err := cursor.All(ctx, &indexes); err != nil {
		t.Fatalf("read indexes: %v", err)
	}

	var slotIndex map[string]any
	for _, index := range indexes {
		if name, _ := index["name"].(string); strings.Contains(name, "slot") {
			slotIndex = index
		}
		t.Logf("bookings index: %v", index["name"])
	}

	if slotIndex == nil {
		t.Fatal("no double-booking index on bookings — two clients can book the same slot")
	}
	if unique, _ := slotIndex["unique"].(bool); !unique {
		t.Error("the slot index is not unique, so it prevents nothing")
	}
	if _, partial := slotIndex["partialFilterExpression"]; !partial {
		t.Error("the slot index is not partial, so a cancelled booking would block its own slot forever")
	}
}

// TestLiveMongoRejectsADoubleBooking is the assertion the entire booking
// model rests on, checked against a real server rather than a fake that
// was written to agree with it.
func TestLiveMongoRejectsADoubleBooking(t *testing.T) {
	client, name := mongoDB(t)
	db := client.Database(name)

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	if err := mongodb.EnsureIndexes(ctx, db); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}

	repo := mongodb.NewBookingRepository(db)
	// Real ObjectIDs. The MongoDB repository stores these as bson.ObjectID
	// and rejects anything else — a constraint the in-memory fakes do not
	// share, which is exactly the kind of divergence this file exists to
	// surface.
	practitioner := bson.NewObjectID().Hex()
	clientA, clientB := bson.NewObjectID().Hex(), bson.NewObjectID().Hex()
	service := bson.NewObjectID().Hex()
	start := time.Now().UTC().Add(30 * 24 * time.Hour).Truncate(time.Hour)

	first, err := domainbooking.New(clientA, practitioner, service, start, 60, time.Now().UTC())
	if err != nil {
		t.Fatalf("domain New: %v", err)
	}
	stored, err := repo.Create(ctx, first)
	if err != nil {
		t.Fatalf("first booking rejected: %v", err)
	}
	t.Cleanup(func() {
		oid, _ := bson.ObjectIDFromHex(practitioner)
		_, _ = db.Collection("bookings").DeleteMany(context.Background(),
			bson.M{"practitionerId": oid})
	})

	// A different client, the same practitioner, the same instant.
	second, err := domainbooking.New(clientB, practitioner, service, start, 60, time.Now().UTC())
	if err != nil {
		t.Fatalf("domain New: %v", err)
	}
	if _, err := repo.Create(ctx, second); err == nil {
		t.Fatal("Atlas accepted a second booking for the same slot — the unique index is not doing its job")
	} else if !errors.Is(err, domainbooking.ErrSlotUnavailable) {
		// The repository must translate the driver's duplicate-key error
		// into the domain's, or the API answers 500 instead of 409.
		t.Errorf("duplicate key surfaced as %v, want ErrSlotUnavailable", err)
	}

	// Round-trip: the bson tags must actually map back to the domain type.
	// A silently-empty field here is the classic adapter bug.
	loaded, err := repo.FindByID(ctx, stored.ID)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if loaded.ClientID != clientA || loaded.PractitionerID != practitioner {
		t.Errorf("parties did not round-trip: %+v", loaded)
	}
	if loaded.ServiceID != service {
		t.Errorf("serviceId did not round-trip: %q", loaded.ServiceID)
	}
	if !loaded.StartAt.Equal(start) {
		t.Errorf("startAt = %v, want %v — a timezone or precision bug in the bson mapping", loaded.StartAt, start)
	}
	if loaded.Status != domainbooking.StatusConfirmed {
		t.Errorf("status = %q, want confirmed", loaded.Status)
	}
}

// TestLiveMongoCancelledSlotIsBookableAgain proves the index is partial in
// the way that matters, not merely flagged partial.
func TestLiveMongoCancelledSlotIsBookableAgain(t *testing.T) {
	client, name := mongoDB(t)
	db := client.Database(name)

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	if err := mongodb.EnsureIndexes(ctx, db); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}

	repo := mongodb.NewBookingRepository(db)
	practitioner := bson.NewObjectID().Hex()
	clientA, clientB := bson.NewObjectID().Hex(), bson.NewObjectID().Hex()
	service := bson.NewObjectID().Hex()
	start := time.Now().UTC().Add(30 * 24 * time.Hour).Truncate(time.Hour)
	now := time.Now().UTC()

	t.Cleanup(func() {
		oid, _ := bson.ObjectIDFromHex(practitioner)
		_, _ = db.Collection("bookings").DeleteMany(context.Background(),
			bson.M{"practitionerId": oid})
	})

	first, _ := domainbooking.New(clientA, practitioner, service, start, 60, now)
	stored, err := repo.Create(ctx, first)
	if err != nil {
		t.Fatalf("first booking: %v", err)
	}

	if err := stored.Cancel(now); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := repo.Update(ctx, stored); err != nil {
		t.Fatalf("persist cancellation: %v", err)
	}

	// The freed slot must be bookable. If the index is not partial, this
	// fails and a cancelled session blocks its own hour forever.
	second, _ := domainbooking.New(clientB, practitioner, service, start, 60, now)
	if _, err := repo.Create(ctx, second); err != nil {
		t.Errorf("a cancelled slot could not be rebooked: %v", err)
	}
}
