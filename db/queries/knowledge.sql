-- Knowledge base. Published content is readable by every signed-in user; drafts and the
-- review workflow only by administrators (enforced in the service and router).

-- name: CreateKnowledgeSource :one
INSERT INTO knowledge_sources (title, publisher, url, published_on, accessed_on, created_by, created_at)
VALUES (@title, sqlc.narg(publisher), sqlc.narg(url), sqlc.narg(published_on), sqlc.narg(accessed_on), @created_by, @now)
RETURNING id, title, publisher, url, published_on, accessed_on, created_at;

-- name: ListKnowledgeSources :many
SELECT id, title, publisher, url, published_on, accessed_on, created_at
FROM knowledge_sources ORDER BY lower(title), id LIMIT 1000;

-- name: CountKnowledgeSources :one
SELECT count(*) FROM knowledge_sources WHERE id = ANY (sqlc.arg(ids)::uuid[]);

-- name: CreateKnowledgeArticle :one
INSERT INTO knowledge_articles (slug, category, created_by, created_at)
VALUES (@slug, @category, @created_by, @now)
ON CONFLICT (slug) DO NOTHING
RETURNING id, slug, category, created_at, retired_at;

-- name: GetKnowledgeArticle :one
SELECT id, slug, category, created_at, retired_at FROM knowledge_articles WHERE id = @id;

-- name: LockKnowledgeArticle :one
SELECT id, slug, category, created_at, retired_at FROM knowledge_articles WHERE id = @id FOR NO KEY UPDATE;

-- name: ListKnowledgeArticlesAdmin :many
SELECT id, slug, category, created_at, retired_at FROM knowledge_articles ORDER BY slug LIMIT 2000;

-- name: ListLatestKnowledgeVersions :many
-- Each article's newest version per locale (the editor's overview).
SELECT DISTINCT ON (article_id, locale) id, article_id, locale, version_no, title, status, updated_at
FROM knowledge_article_versions
WHERE article_id = ANY (sqlc.arg(article_ids)::uuid[])
ORDER BY article_id, locale, version_no DESC;

-- name: ListArticleVersions :many
SELECT id, article_id, locale, version_no, title, summary, claim_class, status, author_id, submitted_at,
       reviewed_by, reviewed_at, review_note, published_at, retired_at, created_at, updated_at, version
FROM knowledge_article_versions WHERE article_id = @article_id
ORDER BY locale, version_no DESC;

-- name: NextKnowledgeVersionNo :one
-- Run with the article locked.
SELECT (COALESCE(max(version_no), 0) + 1)::integer FROM knowledge_article_versions
WHERE article_id = @article_id AND locale = @locale;

-- name: CreateKnowledgeVersion :one
INSERT INTO knowledge_article_versions (article_id, locale, version_no, title, summary, body_md, claim_class,
                                        parameters, symptoms, structured, author_id, created_at, updated_at)
VALUES (@article_id, @locale, @version_no, @title, @summary, @body_md, @claim_class, @parameters, @symptoms,
        @structured, @author_id, @now, @now)
RETURNING id;

-- name: GetKnowledgeVersion :one
SELECT v.id, v.article_id, a.slug, a.category, v.locale, v.version_no, v.title, v.summary, v.body_md,
       v.claim_class, v.parameters, v.symptoms, v.structured, v.status, v.author_id, v.submitted_at,
       v.reviewed_by, v.reviewed_at, v.review_note, v.published_at, v.retired_at, v.created_at, v.updated_at,
       v.version
FROM knowledge_article_versions v
JOIN knowledge_articles a ON a.id = v.article_id
WHERE v.id = @id;

-- name: LockKnowledgeVersion :one
SELECT id, article_id, locale, version_no, status, author_id, claim_class, structured, version
FROM knowledge_article_versions WHERE id = @id FOR NO KEY UPDATE;

-- name: NewerVersionLive :one
-- Whether a later version of the same article and locale was already published (publishing an
-- older one would roll readers back).
SELECT EXISTS (SELECT 1 FROM knowledge_article_versions
               WHERE article_id = @article_id AND locale = @locale AND version_no > @version_no
                 AND status IN ('published', 'retired'));

-- name: InsertKnowledgeReview :exec
INSERT INTO knowledge_version_reviews (version_id, reviewer_id, decision, note, created_at)
VALUES (@version_id, @reviewer_id, @decision, sqlc.narg(note), @now);

-- name: ListKnowledgeReviews :many
SELECT version_id, reviewer_id, decision, note, created_at FROM knowledge_version_reviews
WHERE version_id = @version_id ORDER BY created_at, id;

-- name: UpdateKnowledgeDraft :execrows
UPDATE knowledge_article_versions
SET title = @title, summary = @summary, body_md = @body_md, claim_class = @claim_class, parameters = @parameters,
    symptoms = @symptoms, structured = @structured, updated_at = @now, version = version + 1
WHERE id = @id AND version = @version AND status = 'draft';

-- name: SetKnowledgeVersionSources :exec
-- Replace a draft's sources (the trigger rejects this once submitted).
DELETE FROM knowledge_version_sources WHERE version_id = @version_id;

-- name: AddKnowledgeVersionSource :exec
INSERT INTO knowledge_version_sources (version_id, source_id, note) VALUES (@version_id, @source_id, sqlc.narg(note));

-- name: ListVersionSources :many
SELECT vs.version_id, s.id, s.title, s.publisher, s.url, s.published_on, s.accessed_on, vs.note
FROM knowledge_version_sources vs
JOIN knowledge_sources s ON s.id = vs.source_id
WHERE vs.version_id = ANY (sqlc.arg(version_ids)::uuid[])
ORDER BY vs.version_id, lower(s.title);

-- name: SubmitKnowledgeVersion :execrows
UPDATE knowledge_article_versions
SET status = 'in_review', submitted_at = sqlc.arg(now)::timestamptz, updated_at = @now, version = version + 1
WHERE id = @id AND status = 'draft';

-- name: RejectKnowledgeVersion :execrows
UPDATE knowledge_article_versions
SET status = 'draft', reviewed_by = @reviewer, reviewed_at = sqlc.arg(now)::timestamptz, review_note = @note, updated_at = @now,
    version = version + 1
WHERE id = @id AND status = 'in_review';

-- name: RetirePublishedVersion :exec
-- The previous live version of the same article and locale, when a newer one is published.
UPDATE knowledge_article_versions
SET status = 'retired', retired_at = sqlc.arg(now)::timestamptz, updated_at = @now
WHERE article_id = @article_id AND locale = @locale AND status = 'published';

-- name: PublishKnowledgeVersion :execrows
UPDATE knowledge_article_versions
SET status = 'published', reviewed_by = @reviewer, reviewed_at = sqlc.arg(now)::timestamptz, review_note = sqlc.narg(note),
    published_at = sqlc.arg(now)::timestamptz, updated_at = @now, version = version + 1
WHERE id = @id AND status = 'in_review';

-- name: RetireKnowledgeArticle :execrows
UPDATE knowledge_articles SET retired_at = sqlc.arg(now)::timestamptz WHERE id = @id AND retired_at IS NULL;

-- name: RetireArticleVersions :exec
UPDATE knowledge_article_versions
SET status = 'retired', retired_at = sqlc.arg(now)::timestamptz, updated_at = @now
WHERE article_id = @article_id AND status = 'published';

-- name: ListPublishedKnowledge :many
-- Live articles for readers: each article's published version in the preferred locale, else
-- in the other one, then filtered (so a reader never gets the other locale only because it
-- matched). Sorted by title; capped at 500 (a curated knowledge base stays far smaller).
SELECT x.article_id, x.slug, x.category, x.version_id, x.locale, x.version_no, x.title, x.summary, x.claim_class,
       x.parameters, x.symptoms, x.published_at::timestamptz AS published_at
FROM (
    SELECT DISTINCT ON (a.id) a.id AS article_id, a.slug, a.category, v.id AS version_id, v.locale, v.version_no,
           v.title, v.summary, v.claim_class, v.parameters, v.symptoms, v.published_at
    FROM knowledge_articles a
    JOIN knowledge_article_versions v ON v.article_id = a.id AND v.status = 'published'
    WHERE a.retired_at IS NULL
    ORDER BY a.id, (v.locale = sqlc.arg(locale)::text) DESC
) x
WHERE (cardinality(sqlc.arg(parameters)::text[]) = 0 OR x.parameters && sqlc.arg(parameters)::text[])
  AND (lower(x.title) LIKE sqlc.arg(pattern)::text OR lower(x.summary) LIKE sqlc.arg(pattern)::text)
ORDER BY lower(x.title), x.article_id
LIMIT 500;

-- name: GetPublishedKnowledgeBySlug :one
SELECT v.id, v.article_id, a.slug, a.category, v.locale, v.version_no, v.title, v.summary, v.body_md,
       v.claim_class, v.parameters, v.symptoms, v.structured, v.published_at
FROM knowledge_articles a
JOIN knowledge_article_versions v ON v.article_id = a.id AND v.status = 'published'
WHERE a.slug = @slug AND a.retired_at IS NULL
ORDER BY (v.locale = sqlc.arg(locale)::text) DESC
LIMIT 1;
