// Package knowledge is the curated knowledge base (docs/knowledge-engine.md §2,
// milestone 3.1): articles with immutable, reviewed versions per locale, and their sources.
//
// Business rules:
//   - A version moves draft → in_review → published; a published version becomes retired when
//     a newer one is published or the article is withdrawn. Rejecting sends it back to draft.
//   - Content is frozen once submitted (database trigger): a change is a new version, so an
//     assessment that cites a version always shows what it cited.
//   - Publishing needs an administrator other than the author (four eyes, also a CHECK).
//   - An 'established' version must cite at least one source. Sources are entered by people
//     and are never generated; claims that can't be backed by one are 'unverified'.
//   - Only 'established' and 'expert' content may back recommendations; 'community' and
//     'unverified' may only appear as labelled alternatives (enforced by the intelligence
//     engine).
//   - Readers see published versions only, in their locale with a fallback to the other one.
package knowledge

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/textcheck"
	"reefcare/internal/waterparameters"
)

// Limits.
const (
	maxListItems = 20
	maxItemText  = 500
	maxBody      = 50000
	maxTags      = 20
)

var (
	slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	keyRe  = regexp.MustCompile(`^[a-z][a-z0-9_]{1,59}$`)

	categories = map[string]bool{
		"water_chemistry": true, "testing": true, "dosing": true, "livestock_health": true,
		"equipment": true, "maintenance": true, "general": true,
	}
	claimClasses = map[string]bool{"established": true, "expert": true, "community": true, "unverified": true}
	locales      = map[string]bool{"vi": true, "en": true}
)

// Statuses.
const (
	StatusDraft     = "draft"
	StatusInReview  = "in_review"
	StatusPublished = "published"
	StatusRetired   = "retired"
)

// Source is a citable reference.
type Source struct {
	ID          uuid.UUID
	Title       string
	Publisher   *string
	URL         *string
	PublishedOn *time.Time
	AccessedOn  *time.Time
	Note        *string // what the version cites from it
	CreatedAt   time.Time
}

// Item is one entry of a structured list; Key lets rules refer to it.
type Item struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

// Recommendation is an Item with its own claim class.
type Recommendation struct {
	Key        string `json:"key"`
	Text       string `json:"text"`
	ClaimClass string `json:"claim_class"`
}

// Structured holds the typed lists used by rules.
type Structured struct {
	Verifications     []Item           `json:"verifications"`
	Recommendations   []Recommendation `json:"recommendations"`
	Contraindications []Item           `json:"contraindications"`
	SafetyWarnings    []Item           `json:"safety_warnings"`
}

// Article is a knowledge article (one slug, all locales).
type Article struct {
	ID        uuid.UUID
	Slug      string
	Category  string
	CreatedAt time.Time
	RetiredAt *time.Time
	Latest    []VersionSummary // newest version per locale (editor view)
}

// VersionSummary is a version without its body.
type VersionSummary struct {
	ID        uuid.UUID
	Locale    string
	VersionNo int32
	Title     string
	Status    string
	UpdatedAt time.Time
}

// Version is one locale's version of an article.
type Version struct {
	ID          uuid.UUID
	ArticleID   uuid.UUID
	Slug        string
	Category    string
	Locale      string
	VersionNo   int32
	Title       string
	Summary     string
	BodyMD      string
	ClaimClass  string
	Parameters  []string
	Symptoms    []string
	Structured  Structured
	Status      string
	AuthorID    *uuid.UUID
	SubmittedAt *time.Time
	ReviewedBy  *uuid.UUID
	ReviewedAt  *time.Time
	ReviewNote  *string
	PublishedAt *time.Time
	RetiredAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Revision    int32 // optimistic lock for draft edits
	Sources     []Source
	Reviews     []Review // review history (editors only)
}

// Review is one review decision on a version.
type Review struct {
	ReviewerID *uuid.UUID
	Decision   string // rejected | published
	Note       *string
	At         time.Time
}

// --- validation ---

type checker struct{ v *apperr.Validator }

func (c checker) line(field, s string, minLen, maxLen int) string {
	t := strings.TrimSpace(s)
	if n := utf8.RuneCountInString(t); n < minLen || n > maxLen {
		c.v.Add(field, "length", map[string]string{"min": strconv.Itoa(minLen), "max": strconv.Itoa(maxLen)})
	} else if !textcheck.SingleLine(t) {
		c.v.Add(field, "invalid_characters", nil)
	}
	return t
}

func (c checker) optLine(field string, s *string, maxLen int) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	t := c.line(field, *s, 1, maxLen)
	return &t
}

func (c checker) multi(field, s string, maxLen int) string {
	t := strings.TrimSpace(s)
	if n := utf8.RuneCountInString(t); n < 1 || n > maxLen {
		c.v.Add(field, "length", map[string]string{"min": "1", "max": strconv.Itoa(maxLen)})
	} else if !textcheck.MultiLine(t) {
		c.v.Add(field, "invalid_characters", nil)
	}
	return t
}

func (c checker) oneOf(field, s string, allowed map[string]bool) string {
	if !allowed[s] {
		c.v.Add(field, "unsupported", nil)
	}
	return s
}

func (c checker) slug(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 80 || !slugRe.MatchString(s) {
		c.v.Add("slug", "invalid_slug", map[string]string{"format": "lowercase-words-with-dashes"})
	}
	return s
}

// parameters accepts catalog codes only, without duplicates.
func (c checker) parameters(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	if len(in) > maxTags {
		c.v.Add("parameters", "too_many", map[string]string{"max": strconv.Itoa(maxTags)})
	}
	for i, p := range in {
		if _, ok := waterparameters.Lookup(p); !ok {
			c.v.Add("parameters."+strconv.Itoa(i), "unknown_parameter", nil)
			continue
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// symptoms accepts lower_snake tags, without duplicates.
func (c checker) symptoms(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	if len(in) > maxTags {
		c.v.Add("symptoms", "too_many", map[string]string{"max": strconv.Itoa(maxTags)})
	}
	for i, s := range in {
		if !keyRe.MatchString(s) {
			c.v.Add("symptoms."+strconv.Itoa(i), "invalid_tag", map[string]string{"format": "lower_snake"})
			continue
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func (c checker) items(field string, in []Item) []Item {
	if len(in) > maxListItems {
		c.v.Add(field, "too_many", map[string]string{"max": strconv.Itoa(maxListItems)})
		return nil
	}
	seen := map[string]bool{}
	out := make([]Item, len(in))
	for i, it := range in {
		f := field + "." + strconv.Itoa(i)
		if !keyRe.MatchString(it.Key) {
			c.v.Add(f+".key", "invalid_key", map[string]string{"format": "lower_snake"})
		} else if seen[it.Key] {
			c.v.Add(f+".key", "duplicate", nil)
		}
		seen[it.Key] = true
		out[i] = Item{Key: it.Key, Text: c.line(f+".text", it.Text, 1, maxItemText)}
	}
	return out
}

// structured validates the typed lists and returns their canonical JSON.
func (c checker) structured(s Structured) (Structured, []byte) {
	out := Structured{
		Verifications:     c.items("structured.verifications", s.Verifications),
		Contraindications: c.items("structured.contraindications", s.Contraindications),
		SafetyWarnings:    c.items("structured.safety_warnings", s.SafetyWarnings),
	}
	if len(s.Recommendations) > maxListItems {
		c.v.Add("structured.recommendations", "too_many", map[string]string{"max": strconv.Itoa(maxListItems)})
	} else {
		seen := map[string]bool{}
		for i, r := range s.Recommendations {
			f := "structured.recommendations." + strconv.Itoa(i)
			if !keyRe.MatchString(r.Key) {
				c.v.Add(f+".key", "invalid_key", map[string]string{"format": "lower_snake"})
			} else if seen[r.Key] {
				c.v.Add(f+".key", "duplicate", nil)
			}
			seen[r.Key] = true
			out.Recommendations = append(out.Recommendations, Recommendation{
				Key: r.Key, Text: c.line(f+".text", r.Text, 1, maxItemText), ClaimClass: c.oneOf(f+".claim_class", r.ClaimClass, claimClasses),
			})
		}
	}
	for _, l := range []*[]Item{&out.Verifications, &out.Contraindications, &out.SafetyWarnings} {
		if *l == nil {
			*l = []Item{}
		}
	}
	if out.Recommendations == nil {
		out.Recommendations = []Recommendation{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		c.v.Add("structured", "invalid", nil)
	}
	if buf.Len() > 16000 {
		c.v.Add("structured", "too_large", nil)
	}
	return out, bytes.TrimSpace(buf.Bytes())
}

// likePattern turns a search into a lower-case LIKE pattern with wildcards escaped.
func likePattern(q string) string {
	q = strings.ToLower(strings.TrimSpace(q))
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(q) + "%"
}
