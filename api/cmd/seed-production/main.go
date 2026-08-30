// Command seed-production provisions the named practitioner accounts and the
// approved launch content. It is intentionally separate from cmd/seed: no demo
// records are ever written. SEED_SCOPE=content lets an operator safely repair
// CMS content without placing account passwords in a one-off job command.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/xcreativs/terios/api/internal/adapters/mongodb"
	"github.com/xcreativs/terios/api/internal/adapters/security"
	"github.com/xcreativs/terios/api/internal/config"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const confirmation = "seed-terios-production"

type account struct{ email, name, passwordEnv string }

var accounts = []account{
	{"admin@terioscoach.com", "Terios Administrator", "TERIOS_ADMIN_PASSWORD"},
	{"hayfordstanley@gmail.com", "Hayford Stanley", "TERIOS_OWNER_PASSWORD"},
}

type marketingPage struct{ slug, title, body, coverImage string }

var marketingPages = []marketingPage{
	{"home", "Terios Wellness", "Total wellness is possible. Together, we turn the changes you want into realistic steps that support more health, clarity and vitality.\n\nGrounded in more than two decades of nursing experience, Terios combines clinical understanding with holistic coaching so you can build a way of living well that is genuinely your own.", "/images/brand/theresa-yirerong-clinical.webp"},
	{"about", "About Theresa Yirerong", "I’m Theresa Yirerong. After more than two decades as a registered nurse, I have come to believe that nothing is more valuable than your health and wellbeing.\n\nAfter caring for thousands of people, many with preventable conditions, I reshaped my nursing practice around a simple truth: wellness can be pursued at every stage of life, whether or not you are living with a diagnosis.\n\nTerios gives you one-to-one space to look honestly at where you are, decide what living well means for you, and build realistic steps toward it.", "/images/brand/theresa-yirerong-about.webp"},
	{"work-with-me", "Work with me", "Your care should be as individual as you are. Begin with an obligation-free conversation in a quiet, undistracted space, then choose nurse coaching or holistic coaching shaped around your goals.", "/images/marketing/services-care.webp"},
}

type marketingPost struct {
	slug, title, excerpt, body, coverImage, category string
	tags                                             []string
}

var marketingPosts = []marketingPost{
	{"small-steps-preventive-wellness", "Small steps toward preventive wellness", "Prevention is not perfection. It is the practice of noticing what can change and beginning there.", "Disease and illness are not planned, but many of the patterns that influence wellbeing can be changed over time. Preventive wellness begins by looking honestly at sleep, nourishment, movement, stress, relationships and the support around you.\n\nThe goal is not to overhaul your life overnight. It is to identify one useful change, make it realistic, and build from there. A nurse coach can help you understand the whole picture, choose priorities and stay accountable to plans that belong to you.\n\nCoaching complements medical care; it does not replace diagnosis or treatment. If you are concerned about symptoms or a condition, speak with your licensed healthcare professional.", "/images/blog/woman-8656633_1280.webp", "Wellness", []string{"prevention", "wellness", "habits"}},
	{"coaching-alongside-medical-care", "Coaching alongside your medical care", "How nurse coaching can support the goals you choose while your clinical team manages diagnosis and treatment.", "Living with high blood pressure, diabetes, cancer, heart disease or another long-term condition can make everyday decisions feel complicated. Nurse coaching does not take away the medical care you already receive. It creates protected time to notice where you feel stuck and decide what support would be useful.\n\nTogether, we can explore routines, questions for your clinical team, sources of stress, and goals that feel achievable in your real life. The plans are not imposed on you. They are co-created with you, and they can involve the other professionals in your care when appropriate and with your permission.\n\nAlways seek urgent or diagnostic care from the appropriate licensed medical professional. Coaching is a supportive relationship, not emergency or specialist treatment.", "/images/blog/people-8577400_1280.webp", "Nurse coaching", []string{"nurse coaching", "chronic conditions", "support"}},
	{"a-gentle-beginning-with-mindfulness", "A gentle beginning with mindfulness", "You do not have to feel completely ready before you begin making room for calm.", "Mindfulness is a way of meeting the present moment with attention rather than judgment. It may be as simple as noticing your breath, the sounds around you, or the feeling of your feet on the ground.\n\nWith practice, these small pauses can help you recognize thoughts and feelings without being carried away by every one of them. A guided session offers a safe place to practise, reflect and discover which approaches feel natural to you.\n\nMindfulness can support wellbeing, but it is not a substitute for mental-health or medical treatment. If a practice feels distressing, stop and seek guidance from an appropriately qualified professional.", "/images/blog/lavender-3605688_1280.webp", "Mindfulness", []string{"mindfulness", "meditation", "calm"}},
}

type suppliedTestimonial struct {
	name, role, quote, status string
	order                     int
}

var suppliedTestimonials = []suppliedTestimonial{
	{"Toni Cirrincione", "Mindfulness coaching client — consent supplied", "Theresa is truly a breath of fresh air—comforting and easy to connect with. She created a safe space for me to relax and express my feelings openly. Our mindfulness work helped me reshape difficult thoughts and respond with greater calm. I wholeheartedly recommend working with Theresa.", "approved", 0},
	{"Laura Kurzweil", "Nurse coaching client — confirm publication consent", "Theresa truly listened and took time to understand the complete picture of my life. She helped me set realistic, achievable goals around healthy eating, mindfulness, parenting and preventing burnout. I felt safe being vulnerable without judgment and gained positive routines that work for me.", "pending", 1},
	{"Teresa Cantilli", "Coaching client — confirm publication consent", "The coaching relationship was caring and encouraging. With Theresa’s help I found creative, achievable ways to work on exercise, diet and my environment. I have hope that I can continue these goals through small steps I can accomplish.", "pending", 2},
	{"Jennifer Cantili", "Coaching client — confirm publication consent", "Theresa’s compassionate, non-judgmental and honest approach helped me set boundaries, reduce overwhelm and communicate more effectively. She created a comfortable space where I felt safe opening up.", "pending", 3},
	{"Ashley", "Coaching client — confirm publication consent", "Theresa provided a sympathetic ear and adapted to my needs as a client. I was able to tend to areas of my life that fed into my main struggle and helped me find peace. She loves what she does and it was a pleasure to work with her.", "pending", 4},
	{"Christina Jimenez", "Mindfulness client — confirm publication consent", "Our mindfulness sessions were a safe, cozy space to practise and collaborate. Theresa’s insight and feedback were invaluable; her tone and cadence helped me feel calm and light, and her joyful laughter lifted my spirit.", "pending", 5},
	{"Stephanie Black", "Coaching client — confirm publication consent", "Theresa listened with acute attention to what I was and was not saying. Her questions helped me find answers and navigate a difficult work environment with greater peace and strength.", "pending", 6},
}

func main() {
	if err := run(); err != nil {
		slog.Error("production seed failed", "error", err)
		os.Exit(1)
	}
	slog.Info("production seed complete")
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.Env != "production" {
		return errors.New("refusing production seed: APP_ENV must equal production")
	}
	if os.Getenv("CONFIRM_PRODUCTION_SEED") != confirmation {
		return fmt.Errorf("refusing production seed: set CONFIRM_PRODUCTION_SEED=%s", confirmation)
	}
	scope, err := seedScope(os.Getenv("SEED_SCOPE"))
	if err != nil {
		return err
	}
	if cfg.MongoURI == "" {
		return errors.New("MONGODB_URI is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client, err := mongodb.Connect(ctx, cfg.MongoURI)
	if err != nil {
		return fmt.Errorf("connect MongoDB: %w", err)
	}
	defer client.Disconnect(context.Background())
	db := client.Database(cfg.MongoDBName)
	if err := mongodb.EnsureIndexes(ctx, db); err != nil {
		return fmt.Errorf("ensure indexes: %w", err)
	}
	if scope == "all" {
		for _, a := range accounts {
			if err := ensureAccount(ctx, db, a); err != nil {
				return err
			}
		}
	}
	if err := ensureMarketingPages(ctx, db); err != nil {
		return err
	}
	if err := ensureMarketingPosts(ctx, db); err != nil {
		return err
	}
	if err := ensureTestimonials(ctx, db); err != nil {
		return err
	}
	return nil
}

func seedScope(value string) (string, error) {
	if value == "" {
		return "all", nil
	}
	if value != "all" && value != "content" {
		return "", errors.New("SEED_SCOPE must be all or content")
	}
	return value, nil
}

func ensureMarketingPages(ctx context.Context, db *mongo.Database) error {
	now := time.Now().UTC()
	for _, page := range marketingPages {
		_, err := db.Collection("cms_pages").UpdateOne(ctx,
			bson.M{"slug": page.slug},
			bson.M{"$setOnInsert": bson.M{"slug": page.slug, "title": page.title, "body": page.body, "coverImage": page.coverImage, "status": "published", "publishedAt": now, "createdAt": now, "updatedAt": now}},
			options.UpdateOne().SetUpsert(true),
		)
		if err != nil {
			return fmt.Errorf("provision marketing page %s: %w", page.slug, err)
		}
	}
	return nil
}

func ensureMarketingPosts(ctx context.Context, db *mongo.Database) error {
	now := time.Now().UTC()
	for _, post := range marketingPosts {
		_, err := db.Collection("blog_posts").UpdateOne(ctx, bson.M{"slug": post.slug}, bson.M{"$setOnInsert": bson.M{
			"slug": post.slug, "title": post.title, "excerpt": post.excerpt, "body": post.body, "coverImage": post.coverImage,
			"category": post.category, "tags": post.tags, "metaTitle": post.title + " | Terios Wellness Spa", "metaDescription": post.excerpt,
			"status": "published", "publishedAt": now, "createdAt": now, "updatedAt": now,
		}}, options.UpdateOne().SetUpsert(true))
		if err != nil {
			return fmt.Errorf("provision marketing post %s: %w", post.slug, err)
		}
	}
	return nil
}

func ensureTestimonials(ctx context.Context, db *mongo.Database) error {
	now := time.Now().UTC()
	for _, item := range suppliedTestimonials {
		doc := bson.M{"authorName": item.name, "authorRole": item.role, "quote": item.quote, "status": item.status, "sortOrder": item.order, "submittedAt": now, "createdAt": now, "updatedAt": now}
		if item.status == "approved" {
			doc["approvedAt"] = now
		}
		_, err := db.Collection("testimonials").UpdateOne(ctx, bson.M{"authorName": item.name, "quote": item.quote}, bson.M{"$setOnInsert": doc}, options.UpdateOne().SetUpsert(true))
		if err != nil {
			return fmt.Errorf("provision testimonial %s: %w", item.name, err)
		}
	}
	return nil
}

func ensureAccount(ctx context.Context, db *mongo.Database, a account) error {
	password := os.Getenv(a.passwordEnv)
	if err := identity.ValidatePassword(password); err != nil {
		return fmt.Errorf("%s: %w", a.passwordEnv, err)
	}
	hash, err := security.NewArgon2Hasher().Hash(password)
	if err != nil {
		return fmt.Errorf("hash %s: %w", a.email, err)
	}
	now := time.Now().UTC()
	res, err := db.Collection("users").UpdateOne(ctx,
		bson.M{"email": identity.NormalizeEmail(a.email)},
		bson.M{"$setOnInsert": bson.M{"email": identity.NormalizeEmail(a.email), "passwordHash": hash, "role": string(identity.RolePractitioner), "name": a.name, "active": true, "createdAt": now, "updatedAt": now, "mfaEnabled": false}},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("provision %s: %w", a.email, err)
	}
	if res.UpsertedCount == 0 {
		slog.Info("practitioner already exists; password and MFA state preserved", "email", a.email)
	} else {
		slog.Info("created practitioner", "email", a.email)
	}
	return nil
}
