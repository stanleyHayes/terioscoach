package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/xcreativs/terios/api/internal/adapters/security"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Demo data: enough of a practice that every screen in both apps has
// something real in it.
//
// It is deliberately not random. Each row exists to put one screen into a
// specific state worth looking at — a session starting in five minutes so
// the video room can actually be opened, an unpaid booking so the Pay
// button appears, a pending testimonial so moderation has something to
// moderate. Random data fills screens; chosen data exercises them.
//
// Re-running is safe: everything it creates is removed first, keyed on the
// demo client accounts.

// demoClient is one seeded client account.
type demoClient struct {
	email string
	name  string
	phone string
	tags  []string
}

var demoClients = []demoClient{
	{"ama@example.com", "Ama Serwaa", "+233 20 123 4567", []string{"regular", "deep tissue"}},
	{"kofi@example.com", "Kofi Mensah", "+233 24 987 6543", []string{"new"}},
	{"efua@example.com", "Efua Boateng", "+233 26 555 0110", nil},
}

// seedDemo fills the practice with usable data.
func seedDemo(ctx context.Context, db *mongo.Database, practitionerID bson.ObjectID) error {
	now := time.Now().UTC()

	clientIDs, err := seedDemoClients(ctx, db, now)
	if err != nil {
		return err
	}
	if err := clearPreviousDemo(ctx, db, clientIDs); err != nil {
		return err
	}

	services, err := practitionerServices(ctx, db, practitionerID)
	if err != nil {
		return err
	}
	if len(services) < 2 {
		return fmt.Errorf("expected at least 2 seeded services, found %d", len(services))
	}

	if err := seedDemoBookings(ctx, db, practitionerID, clientIDs, services, now); err != nil {
		return err
	}
	if err := seedDemoContent(ctx, db, now); err != nil {
		return err
	}
	if err := seedDemoEnquiries(ctx, db, now); err != nil {
		return err
	}
	return nil
}

// seedDemoClients upserts the client accounts and their profiles.
func seedDemoClients(ctx context.Context, db *mongo.Database, now time.Time) (map[string]bson.ObjectID, error) {
	hash, err := security.NewArgon2Hasher().Hash(seedPassword)
	if err != nil {
		return nil, fmt.Errorf("hash client password: %w", err)
	}

	ids := make(map[string]bson.ObjectID, len(demoClients))
	for _, client := range demoClients {
		if _, err := db.Collection("users").UpdateOne(ctx,
			bson.M{"email": client.email},
			bson.M{"$set": bson.M{
				"email":        client.email,
				"passwordHash": hash,
				"role":         "client",
				"name":         client.name,
				"createdAt":    now.Add(-90 * 24 * time.Hour),
			}},
			options.UpdateOne().SetUpsert(true),
		); err != nil {
			return nil, fmt.Errorf("upsert client %s: %w", client.email, err)
		}

		var user struct {
			ID bson.ObjectID `bson:"_id"`
		}
		if err := db.Collection("users").FindOne(ctx, bson.M{"email": client.email}).Decode(&user); err != nil {
			return nil, fmt.Errorf("lookup client %s: %w", client.email, err)
		}
		ids[client.email] = user.ID

		if _, err := db.Collection("client_profiles").UpdateOne(ctx,
			bson.M{"userId": user.ID},
			bson.M{"$set": bson.M{
				"userId":        user.ID,
				"phone":         client.phone,
				"tags":          orEmpty(client.tags),
				"practiceNotes": "",
				"createdAt":     now.Add(-90 * 24 * time.Hour),
				"updatedAt":     now,
			}},
			options.UpdateOne().SetUpsert(true),
		); err != nil {
			return nil, fmt.Errorf("upsert profile %s: %w", client.email, err)
		}
	}
	slog.Info("seeded demo clients", "count", len(ids), "password", seedPassword)
	return ids, nil
}

// clearPreviousDemo removes what an earlier run created, so re-seeding does
// not pile up duplicate bookings on the calendar.
func clearPreviousDemo(ctx context.Context, db *mongo.Database, clientIDs map[string]bson.ObjectID) error {
	ids := make([]bson.ObjectID, 0, len(clientIDs))
	for _, id := range clientIDs {
		ids = append(ids, id)
	}
	owned := bson.M{"clientId": bson.M{"$in": ids}}

	var bookingIDs []bson.ObjectID
	cursor, err := db.Collection("bookings").Find(ctx, owned)
	if err != nil {
		return fmt.Errorf("find previous bookings: %w", err)
	}
	var found []struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := cursor.All(ctx, &found); err != nil {
		return fmt.Errorf("read previous bookings: %w", err)
	}
	for _, b := range found {
		bookingIDs = append(bookingIDs, b.ID)
	}

	for _, collection := range []string{"bookings", "payments", "reviews"} {
		if _, err := db.Collection(collection).DeleteMany(ctx, owned); err != nil {
			return fmt.Errorf("clear %s: %w", collection, err)
		}
	}
	if len(bookingIDs) > 0 {
		if _, err := db.Collection("session_notes").DeleteMany(ctx,
			bson.M{"bookingId": bson.M{"$in": bookingIDs}}); err != nil {
			return fmt.Errorf("clear session_notes: %w", err)
		}
	}
	return nil
}

// practitionerServices returns the practitioner's services, newest last.
func practitionerServices(ctx context.Context, db *mongo.Database, practitionerID bson.ObjectID) ([]struct {
	ID          bson.ObjectID `bson:"_id"`
	Name        string        `bson:"name"`
	DurationMin int           `bson:"durationMin"`
	PriceKobo   int64         `bson:"priceKobo"`
	Currency    string        `bson:"currency"`
}, error) {
	var services []struct {
		ID          bson.ObjectID `bson:"_id"`
		Name        string        `bson:"name"`
		DurationMin int           `bson:"durationMin"`
		PriceKobo   int64         `bson:"priceKobo"`
		Currency    string        `bson:"currency"`
	}
	cursor, err := db.Collection("services").Find(ctx,
		bson.M{"practitionerId": practitionerID},
		options.Find().SetSort(bson.D{{Key: "sortOrder", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find services: %w", err)
	}
	if err := cursor.All(ctx, &services); err != nil {
		return nil, fmt.Errorf("read services: %w", err)
	}
	return services, nil
}

// seedDemoBookings writes the sessions, their payments, notes and reviews.
func seedDemoBookings(
	ctx context.Context,
	db *mongo.Database,
	practitionerID bson.ObjectID,
	clients map[string]bson.ObjectID,
	services []struct {
		ID          bson.ObjectID `bson:"_id"`
		Name        string        `bson:"name"`
		DurationMin int           `bson:"durationMin"`
		PriceKobo   int64         `bson:"priceKobo"`
		Currency    string        `bson:"currency"`
	},
	now time.Time,
) error {
	swedish, deep := services[0], services[1]
	ama, kofi, efua := clients["ama@example.com"], clients["kofi@example.com"], clients["efua@example.com"]

	// nextWeekday returns a working-hours slot n days out, at 10:00 UTC —
	// inside the seeded Mon–Fri 09:00–17:00 window.
	slot := func(days int, hour int) time.Time {
		day := now.AddDate(0, 0, days).Truncate(24 * time.Hour)
		for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			day = day.AddDate(0, 0, 1)
		}
		return day.Add(time.Duration(hour) * time.Hour)
	}

	type seededBooking struct {
		id            bson.ObjectID
		clientID      bson.ObjectID
		serviceID     bson.ObjectID
		start, end    time.Time
		status        string
		paymentStatus string
	}

	// The session starting in five minutes is the important one: the video
	// room opens ten minutes before, so this is the only row that lets
	// anyone actually click Join and land in a live room.
	imminentStart := now.Add(5 * time.Minute)

	rows := []seededBooking{
		{bson.NewObjectID(), ama, deep.ID, imminentStart, imminentStart.Add(time.Duration(deep.DurationMin) * time.Minute), "confirmed", "paid"},
		{bson.NewObjectID(), ama, swedish.ID, slot(-14, 10), slot(-14, 10).Add(time.Duration(swedish.DurationMin) * time.Minute), "completed", "paid"},
		{bson.NewObjectID(), kofi, swedish.ID, slot(3, 11), slot(3, 11).Add(time.Duration(swedish.DurationMin) * time.Minute), "confirmed", ""},
		{bson.NewObjectID(), kofi, deep.ID, slot(-30, 14), slot(-30, 14).Add(time.Duration(deep.DurationMin) * time.Minute), "completed", "paid"},
		{bson.NewObjectID(), efua, swedish.ID, slot(7, 15), slot(7, 15).Add(time.Duration(swedish.DurationMin) * time.Minute), "cancelled", ""},
		{bson.NewObjectID(), efua, swedish.ID, slot(-7, 9), slot(-7, 9).Add(time.Duration(swedish.DurationMin) * time.Minute), "no_show", "paid"},
	}

	docs := make([]any, 0, len(rows))
	for _, row := range rows {
		doc := bson.M{
			"_id":            row.id,
			"clientId":       row.clientID,
			"practitionerId": practitionerID,
			"serviceId":      row.serviceID,
			"startAt":        row.start,
			"endAt":          row.end,
			"status":         row.status,
			"createdAt":      row.start.Add(-10 * 24 * time.Hour),
			"updatedAt":      now,
		}
		if row.paymentStatus != "" {
			doc["paymentStatus"] = row.paymentStatus
			doc["paidAt"] = row.start.Add(-9 * 24 * time.Hour)
		}
		if row.status == "cancelled" {
			doc["cancelledAt"] = now.Add(-2 * 24 * time.Hour)
		}
		if row.status == "completed" || row.status == "no_show" {
			doc["completedAt"] = row.end
		}
		docs = append(docs, doc)
	}
	if _, err := db.Collection("bookings").InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("insert bookings: %w", err)
	}

	// Payments. One successful, one still pending (so the Pay button has
	// something to pick up), one refunded.
	payments := []any{
		payment(rows[0].id, ama, deep.PriceKobo, deep.Currency, "success", now.Add(-9*24*time.Hour), now),
		payment(rows[1].id, ama, swedish.PriceKobo, swedish.Currency, "success", rows[1].start.Add(-9*24*time.Hour), now),
		payment(rows[2].id, kofi, swedish.PriceKobo, swedish.Currency, "pending", time.Time{}, now),
		payment(rows[3].id, kofi, deep.PriceKobo, deep.Currency, "refunded", rows[3].start.Add(-9*24*time.Hour), now),
		payment(rows[5].id, efua, swedish.PriceKobo, swedish.Currency, "success", rows[5].start.Add(-9*24*time.Hour), now),
	}
	if _, err := db.Collection("payments").InsertMany(ctx, payments); err != nil {
		return fmt.Errorf("insert payments: %w", err)
	}

	// Session notes: one shared with the client, one kept private, so both
	// sides of the private/shared split are visible.
	notes := []any{
		bson.M{
			"_id":             bson.NewObjectID(),
			"bookingId":       rows[1].id,
			"clientId":        ama,
			"practitionerId":  practitionerID,
			"privateNotes":    "Chronic tension across the upper trapezius. New desk job — worth revisiting posture next time.",
			"sharedFeedback":  "A really good session. Your shoulders released a lot more easily than last time, which is a good sign.",
			"sharedResources": []string{"Ten minutes of the doorway stretch we did, daily.", "Keep the water up for the next 48 hours."},
			"sharedAt":        rows[1].end.Add(2 * time.Hour),
			"createdAt":       rows[1].end,
			"updatedAt":       rows[1].end.Add(2 * time.Hour),
		},
		bson.M{
			"_id":             bson.NewObjectID(),
			"bookingId":       rows[3].id,
			"clientId":        kofi,
			"practitionerId":  practitionerID,
			"privateNotes":    "First session. Guarded through the lower back — go gentler until he settles.",
			"sharedFeedback":  "",
			"sharedResources": []string{},
			"createdAt":       rows[3].end,
			"updatedAt":       rows[3].end,
		},
	}
	if _, err := db.Collection("session_notes").InsertMany(ctx, notes); err != nil {
		return fmt.Errorf("insert session notes: %w", err)
	}

	// Reviews: one approved and public, one waiting on moderation.
	//
	// Note the .Hex(): the reviews collection stores its foreign keys as
	// STRINGS, where bookings, payments, notes and profiles all store
	// ObjectIDs. Writing ObjectIDs here decodes into an error and the
	// public reviews endpoint answers 500.
	reviews := []any{
		bson.M{
			"_id":            bson.NewObjectID(),
			"bookingId":      rows[1].id.Hex(),
			"clientId":       ama.Hex(),
			"practitionerId": practitionerID.Hex(),
			"serviceId":      swedish.ID.Hex(),
			"rating":         5,
			"comment":        "I left feeling like a different person. The best hour of my month.",
			"status":         "approved",
			"moderatedAt":    now.Add(-10 * 24 * time.Hour),
			"createdAt":      rows[1].end.Add(24 * time.Hour),
			"updatedAt":      now.Add(-10 * 24 * time.Hour),
		},
		bson.M{
			"_id":            bson.NewObjectID(),
			"bookingId":      rows[3].id.Hex(),
			"clientId":       kofi.Hex(),
			"practitionerId": practitionerID.Hex(),
			"serviceId":      deep.ID.Hex(),
			"rating":         4,
			"comment":        "Very professional and easy to talk to. Booking was simple.",
			"status":         "pending",
			"createdAt":      rows[3].end.Add(24 * time.Hour),
			"updatedAt":      rows[3].end.Add(24 * time.Hour),
		},
	}
	if _, err := db.Collection("reviews").InsertMany(ctx, reviews); err != nil {
		return fmt.Errorf("insert reviews: %w", err)
	}

	slog.Info("seeded demo bookings",
		"bookings", len(rows), "payments", len(payments), "notes", len(notes), "reviews", len(reviews),
		"joinable_session_starts", imminentStart.Format(time.Kitchen))
	return nil
}

func payment(bookingID, clientID bson.ObjectID, amount int64, currency, status string, paidAt, now time.Time) bson.M {
	doc := bson.M{
		"_id":        bson.NewObjectID(),
		"bookingId":  bookingID,
		"clientId":   clientID,
		"amountKobo": amount,
		"currency":   currency,
		"status":     status,
		// Keyed on the booking, not a fresh ObjectID: ObjectIDs minted in
		// the same second share their first 12 hex characters, so a
		// truncated one is not unique and the reference index rejects it.
		"providerReference": "demo-" + bookingID.Hex(),
		"createdAt":         now.Add(-10 * 24 * time.Hour),
		"updatedAt":         now,
	}
	if !paidAt.IsZero() {
		doc["paidAt"] = paidAt
		doc["channel"] = "card"
	}
	if status == "refunded" {
		doc["refundedAt"] = now.Add(-3 * 24 * time.Hour)
	}
	return doc
}

// seedDemoContent fills the CMS: pages, posts, FAQs and testimonials, with
// a mix of published and awaiting-you so both states are visible.
// Collection names are the repositories' own — cms_pages and blog_posts,
// not the plural of the type. Seeding the obvious name instead writes
// documents nothing ever reads, which looks exactly like a CMS that has
// no content.
func seedDemoContent(ctx context.Context, db *mongo.Database, now time.Time) error {
	if _, err := db.Collection("cms_pages").DeleteMany(ctx, bson.M{}); err != nil {
		return fmt.Errorf("clear pages: %w", err)
	}
	pages := []any{
		bson.M{
			"_id": bson.NewObjectID(), "slug": "about", "title": "About the practice",
			"body":      "Terios is a small wellness practice in Accra.\n\nEvery session is one-to-one, unhurried, and built around what you actually need that week rather than a fixed routine.",
			"metaTitle": "About — Terios Wellness Spa", "metaDescription": "A small one-to-one wellness practice in Accra.",
			"status": "published", "publishedAt": now.Add(-30 * 24 * time.Hour),
			"createdAt": now.Add(-40 * 24 * time.Hour), "updatedAt": now.Add(-30 * 24 * time.Hour),
		},
		bson.M{
			"_id": bson.NewObjectID(), "slug": "cancellation-policy", "title": "Cancellation policy",
			"body":      "Sessions can be moved or cancelled up to 24 hours beforehand from your portal.\n\nInside 24 hours, get in touch and we will work something out.",
			"status":    "draft",
			"createdAt": now.Add(-5 * 24 * time.Hour), "updatedAt": now.Add(-2 * 24 * time.Hour),
		},
	}
	if _, err := db.Collection("cms_pages").InsertMany(ctx, pages); err != nil {
		return fmt.Errorf("insert pages: %w", err)
	}

	if _, err := db.Collection("blog_posts").DeleteMany(ctx, bson.M{}); err != nil {
		return fmt.Errorf("clear posts: %w", err)
	}
	posts := []any{
		bson.M{
			"_id": bson.NewObjectID(), "slug": "five-ways-to-rest-properly", "title": "Five ways to rest properly",
			"excerpt":  "Rest is not the same as stopping. A few things that actually help.",
			"body":     "Most of us are not short of time so much as short of proper rest.\n\nThe difference matters: stopping is the absence of activity, and rest is something you do on purpose.\n\nHere are five things worth trying this week.",
			"category": "Wellbeing", "tags": []string{"rest", "sleep"},
			"status": "published", "publishedAt": now.Add(-12 * 24 * time.Hour),
			"createdAt": now.Add(-14 * 24 * time.Hour), "updatedAt": now.Add(-12 * 24 * time.Hour),
		},
		bson.M{
			"_id": bson.NewObjectID(), "slug": "what-to-expect-first-session", "title": "What to expect from your first session",
			"excerpt":  "Nothing to prepare, and nothing to worry about.",
			"body":     "If you have never had a session before, the not-knowing is usually the hardest part.\n\nHere is exactly what happens, start to finish.",
			"category": "Practice", "tags": []string{"first visit"},
			"status":    "draft",
			"createdAt": now.Add(-3 * 24 * time.Hour), "updatedAt": now.Add(-1 * 24 * time.Hour),
		},
	}
	if _, err := db.Collection("blog_posts").InsertMany(ctx, posts); err != nil {
		return fmt.Errorf("insert posts: %w", err)
	}

	if _, err := db.Collection("faqs").DeleteMany(ctx, bson.M{}); err != nil {
		return fmt.Errorf("clear faqs: %w", err)
	}
	faqs := []any{
		faq("Do I need to bring anything?", "Just yourself. Everything is provided.", "Before your visit", 0, true, now),
		faq("How far ahead should I book?", "A week is usually plenty, though popular evening slots go sooner.", "Booking", 1, true, now),
		faq("Can I move my session?", "Yes — from your portal, up to 24 hours beforehand.", "Booking", 2, true, now),
		faq("Do you offer gift vouchers?", "Not yet, but it is on the list.", "", 3, false, now),
	}
	if _, err := db.Collection("faqs").InsertMany(ctx, faqs); err != nil {
		return fmt.Errorf("insert faqs: %w", err)
	}

	if _, err := db.Collection("testimonials").DeleteMany(ctx, bson.M{}); err != nil {
		return fmt.Errorf("clear testimonials: %w", err)
	}
	testimonials := []any{
		testimonial("Ama S.", "Client since 2024", "I have never slept better than the night after a session here.", "approved", 0, now),
		testimonial("Kofi M.", "", "Calm, professional, and genuinely good at what she does.", "approved", 1, now),
		testimonial("Efua B.", "", "Booked on a whim and it has become the thing I look forward to.", "pending", 2, now),
	}
	if _, err := db.Collection("testimonials").InsertMany(ctx, testimonials); err != nil {
		return fmt.Errorf("insert testimonials: %w", err)
	}

	slog.Info("seeded CMS", "pages", len(pages), "posts", len(posts), "faqs", len(faqs), "testimonials", len(testimonials))
	return nil
}

func faq(question, answer, category string, order int, active bool, now time.Time) bson.M {
	return bson.M{
		"_id": bson.NewObjectID(), "question": question, "answer": answer,
		"category": category, "sortOrder": order, "active": active,
		"createdAt": now.Add(-20 * 24 * time.Hour), "updatedAt": now,
	}
}

func testimonial(name, role, quote, status string, order int, now time.Time) bson.M {
	doc := bson.M{
		"_id": bson.NewObjectID(), "authorName": name, "authorRole": role, "quote": quote,
		"status": status, "sortOrder": order,
		"submittedAt": now.Add(-25 * 24 * time.Hour),
		"createdAt":   now.Add(-25 * 24 * time.Hour), "updatedAt": now,
	}
	if status == "approved" {
		doc["approvedAt"] = now.Add(-24 * 24 * time.Hour)
	}
	return doc
}

// seedDemoEnquiries gives the inbox something to open, one already read so
// both states show.
func seedDemoEnquiries(ctx context.Context, db *mongo.Database, now time.Time) error {
	if _, err := db.Collection("enquiries").DeleteMany(ctx, bson.M{}); err != nil {
		return fmt.Errorf("clear enquiries: %w", err)
	}
	enquiries := []any{
		bson.M{
			"_id": bson.NewObjectID(), "name": "Naa Adjeley", "email": "naa@example.com",
			"phone": "+233 27 444 1212", "subject": "Corporate sessions",
			"message":   "Hello — do you do on-site sessions for small teams? We are eight people in Osu.",
			"status":    "new",
			"sourceIp":  "41.66.0.10",
			"createdAt": now.Add(-6 * time.Hour), "updatedAt": now.Add(-6 * time.Hour),
		},
		bson.M{
			"_id": bson.NewObjectID(), "name": "Yaw Owusu", "email": "yaw@example.com",
			"subject":   "Pregnancy massage",
			"message":   "Is pregnancy massage something you offer? My wife is 22 weeks.",
			"status":    "read",
			"sourceIp":  "154.160.0.44",
			"createdAt": now.Add(-3 * 24 * time.Hour), "updatedAt": now.Add(-2 * 24 * time.Hour),
		},
	}
	if _, err := db.Collection("enquiries").InsertMany(ctx, enquiries); err != nil {
		return fmt.Errorf("insert enquiries: %w", err)
	}
	slog.Info("seeded enquiries", "count", len(enquiries))
	return nil
}

func orEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
