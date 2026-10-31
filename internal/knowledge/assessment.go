package knowledge

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/database"
)

// Cited is a published knowledge version an assessment may cite.
type Cited struct {
	VersionID      uuid.UUID
	Slug           string
	Title          string
	Locale         string
	ClaimClass     string
	SafetyWarnings []Item
}

// PublishedBySlug returns the live version of each slug in locale (or the other locale), in
// the caller's transaction. Slugs without a live article are left out: an assessment never
// cites knowledge that doesn't exist (intelligence-safety rules).
func PublishedBySlug(ctx context.Context, q *dbq.Queries, slugs []string, locale string) (map[string]Cited, error) {
	if !locales[locale] {
		locale = "vi"
	}
	out := map[string]Cited{}
	for _, slug := range slugs {
		if _, done := out[slug]; done {
			continue
		}
		r, err := q.GetPublishedKnowledgeBySlug(ctx, dbq.GetPublishedKnowledgeBySlugParams{Slug: slug, Locale: locale})
		if database.IsNoRows(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var st Structured
		if err := json.Unmarshal(r.Structured, &st); err != nil {
			return nil, fmt.Errorf("decode structured knowledge: %w", err)
		}
		out[slug] = Cited{VersionID: r.ID, Slug: r.Slug, Title: r.Title, Locale: r.Locale, ClaimClass: r.ClaimClass, SafetyWarnings: st.SafetyWarnings}
	}
	return out, nil
}
