-- +goose Up
-- Curated knowledge base (docs/knowledge-engine.md §2, milestone 3.1). Articles have
-- immutable, reviewed versions per locale; assessments cite exact versions.
SET LOCAL lock_timeout = '5s';

-- A citable source. Only real, checkable sources are entered (never invented); a version
-- whose claims can't be backed by one stays 'unverified' and out of recommendation paths.
CREATE TABLE knowledge_sources (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    title        text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 300),
    publisher    text CHECK (char_length(publisher) BETWEEN 1 AND 200),
    url          text CHECK (url ~ '^https://' AND char_length(url) <= 1000),
    published_on date,
    accessed_on  date,
    created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CHECK (url IS NOT NULL OR publisher IS NOT NULL) -- something a reader can look up
);

CREATE TABLE knowledge_articles (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    slug       text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND char_length(slug) <= 80),
    category   text NOT NULL CHECK (category IN ('water_chemistry', 'testing', 'dosing', 'livestock_health',
                                                 'equipment', 'maintenance', 'general')),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    retired_at timestamptz -- the whole article withdrawn; its versions stay citable history
);

-- One version of an article in one locale. Workflow: draft → in_review → published, and a
-- published version becomes retired when a newer one is published or the article is
-- withdrawn. A reviewer other than the author publishes. Content is frozen once submitted
-- (trigger below): a change is a new version, so assessments that cite one never change.
CREATE TABLE knowledge_article_versions (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    article_id   uuid NOT NULL REFERENCES knowledge_articles (id) ON DELETE RESTRICT,
    locale       text NOT NULL CHECK (locale IN ('vi', 'en')),
    version_no   int NOT NULL CHECK (version_no >= 1),
    title        text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    summary      text NOT NULL CHECK (char_length(summary) BETWEEN 1 AND 1000),
    body_md      text NOT NULL CHECK (char_length(body_md) BETWEEN 1 AND 50000),
    -- established: documented science or manufacturer data; expert: reviewed interpretation;
    -- community: reported experience; unverified: hypothesis. Only the first two may back
    -- recommendations.
    claim_class  text NOT NULL CHECK (claim_class IN ('established', 'expert', 'community', 'unverified')),
    parameters   text[] NOT NULL DEFAULT '{}', -- parameter_types codes (validated by the service)
    symptoms     text[] NOT NULL DEFAULT '{}', -- lower_snake tags
    -- Typed lists used by rules: verifications, recommendations (each with its own claim
    -- class), contraindications, safety_warnings. Validated by the service.
    structured   jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(structured) = 'object'
                                                    AND pg_column_size(structured) <= 16384),
    status       text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'in_review', 'published', 'retired')),
    author_id    uuid REFERENCES users (id) ON DELETE SET NULL,
    submitted_at timestamptz,
    reviewed_by  uuid REFERENCES users (id) ON DELETE SET NULL,
    reviewed_at  timestamptz,
    review_note  text CHECK (char_length(review_note) BETWEEN 1 AND 2000),
    published_at timestamptz,
    retired_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    version      int NOT NULL DEFAULT 1, -- optimistic lock for draft edits
    UNIQUE (article_id, locale, version_no),
    CHECK ((status IN ('published', 'retired')) = (published_at IS NOT NULL)),
    CHECK ((status = 'retired') = (retired_at IS NOT NULL)),
    CHECK (status = 'draft' OR submitted_at IS NOT NULL),
    CHECK (published_at IS NULL OR (reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)),
    -- Four eyes: the reviewer is never the author (both may later become NULL on erasure).
    -- Only the author edits and submits a draft (service), and author_id is frozen once
    -- submitted (trigger), so this compares the reviewer with the person who wrote it.
    CHECK (reviewed_by IS NULL OR author_id IS NULL OR reviewed_by <> author_id),
    CHECK (retired_at IS NULL OR published_at IS NULL OR retired_at >= published_at),
    CHECK (published_at IS NULL OR submitted_at IS NULL OR published_at >= submitted_at),
    CHECK (cardinality(parameters) <= 20 AND cardinality(symptoms) <= 20)
);
-- At most one live version per article and locale.
CREATE UNIQUE INDEX knowledge_versions_published ON knowledge_article_versions (article_id, locale)
    WHERE status = 'published';

CREATE TABLE knowledge_version_sources (
    version_id uuid NOT NULL REFERENCES knowledge_article_versions (id) ON DELETE RESTRICT,
    source_id  uuid NOT NULL REFERENCES knowledge_sources (id) ON DELETE RESTRICT,
    note       text CHECK (char_length(note) BETWEEN 1 AND 300), -- e.g. the page or table cited
    PRIMARY KEY (version_id, source_id)
);
CREATE INDEX knowledge_version_sources_by_source ON knowledge_version_sources (source_id);

-- +goose StatementBegin
-- The workflow, enforced in the database too:
--   * a version is created as a draft;
--   * draft → draft | in_review; in_review → draft (rejected) | published;
--     published → retired; retired stays retired;
--   * content (and the author) is frozen once a version leaves draft;
--   * the review record of a published or retired version never changes, except that people
--     may become NULL (ON DELETE SET NULL) and retired_at is set on retirement.
CREATE FUNCTION knowledge_versions_workflow() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.status <> 'draft' OR NEW.submitted_at IS NOT NULL OR NEW.reviewed_by IS NOT NULL
           OR NEW.published_at IS NOT NULL THEN
            RAISE EXCEPTION 'a knowledge version starts as a draft' USING ERRCODE = 'insufficient_privilege';
        END IF;
        RETURN NEW;
    END IF;
    IF NOT ((OLD.status = 'draft' AND NEW.status IN ('draft', 'in_review'))
            OR (OLD.status = 'in_review' AND NEW.status IN ('draft', 'published'))
            OR (OLD.status = 'published' AND NEW.status IN ('published', 'retired'))
            OR (OLD.status = 'retired' AND NEW.status = 'retired'))
    THEN
        RAISE EXCEPTION 'knowledge version % cannot go from % to %', OLD.id, OLD.status, NEW.status
            USING ERRCODE = 'insufficient_privilege';
    END IF;
    IF (NEW.id, NEW.article_id, NEW.locale, NEW.version_no, NEW.created_at)
       IS DISTINCT FROM (OLD.id, OLD.article_id, OLD.locale, OLD.version_no, OLD.created_at)
       OR (NEW.author_id IS DISTINCT FROM OLD.author_id AND NEW.author_id IS NOT NULL)
    THEN
        RAISE EXCEPTION 'knowledge version % identity is frozen', OLD.id USING ERRCODE = 'insufficient_privilege';
    END IF;
    IF OLD.status = 'draft' THEN
        RETURN NEW;
    END IF;
    IF (NEW.title, NEW.summary, NEW.body_md, NEW.claim_class, NEW.parameters, NEW.symptoms, NEW.structured)
       IS DISTINCT FROM
       (OLD.title, OLD.summary, OLD.body_md, OLD.claim_class, OLD.parameters, OLD.symptoms, OLD.structured)
    THEN
        RAISE EXCEPTION 'knowledge version % is frozen', OLD.id USING ERRCODE = 'insufficient_privilege';
    END IF;
    IF OLD.status IN ('published', 'retired')
       AND ((NEW.submitted_at, NEW.reviewed_at, NEW.review_note, NEW.published_at)
                IS DISTINCT FROM (OLD.submitted_at, OLD.reviewed_at, OLD.review_note, OLD.published_at)
            OR (NEW.reviewed_by IS DISTINCT FROM OLD.reviewed_by AND NEW.reviewed_by IS NOT NULL)
            OR (OLD.status = 'retired' AND NEW.retired_at IS DISTINCT FROM OLD.retired_at))
    THEN
        RAISE EXCEPTION 'the review record of knowledge version % is frozen', OLD.id
            USING ERRCODE = 'insufficient_privilege';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER knowledge_versions_workflow
    BEFORE INSERT OR UPDATE ON knowledge_article_versions
    FOR EACH ROW EXECUTE FUNCTION knowledge_versions_workflow();

-- Every review decision, append-only (the version keeps only the latest note).
CREATE TABLE knowledge_version_reviews (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    version_id  uuid NOT NULL REFERENCES knowledge_article_versions (id) ON DELETE RESTRICT,
    reviewer_id uuid REFERENCES users (id) ON DELETE SET NULL,
    decision    text NOT NULL CHECK (decision IN ('rejected', 'published')),
    note        text CHECK (char_length(note) BETWEEN 1 AND 2000),
    created_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (decision = 'published' OR note IS NOT NULL)
);
CREATE INDEX knowledge_version_reviews_by_version ON knowledge_version_reviews (version_id, created_at);

-- +goose StatementBegin
-- Slugs are cited and linked; sources are cited by frozen versions: neither changes.
CREATE FUNCTION knowledge_immutable() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    IF TG_TABLE_NAME = 'knowledge_articles'
       AND (NEW.id, NEW.slug, NEW.category, NEW.created_at) IS NOT DISTINCT FROM (OLD.id, OLD.slug, OLD.category, OLD.created_at)
       AND (OLD.retired_at IS NULL OR NEW.retired_at IS NOT DISTINCT FROM OLD.retired_at)
       AND (NEW.created_by IS NOT DISTINCT FROM OLD.created_by OR NEW.created_by IS NULL) THEN
        RETURN NEW;
    END IF;
    IF TG_TABLE_NAME = 'knowledge_sources'
       AND (NEW.id, NEW.title, NEW.publisher, NEW.url, NEW.published_on, NEW.accessed_on, NEW.created_at)
           IS NOT DISTINCT FROM (OLD.id, OLD.title, OLD.publisher, OLD.url, OLD.published_on, OLD.accessed_on, OLD.created_at)
       AND (NEW.created_by IS NOT DISTINCT FROM OLD.created_by OR NEW.created_by IS NULL) THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION '% rows are immutable', TG_TABLE_NAME USING ERRCODE = 'insufficient_privilege';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER knowledge_articles_immutable
    BEFORE UPDATE ON knowledge_articles
    FOR EACH ROW EXECUTE FUNCTION knowledge_immutable();
CREATE TRIGGER knowledge_sources_immutable
    BEFORE UPDATE ON knowledge_sources
    FOR EACH ROW EXECUTE FUNCTION knowledge_immutable();

-- +goose StatementBegin
-- Sources of a submitted version are frozen too.
CREATE FUNCTION knowledge_version_sources_frozen() RETURNS trigger
    LANGUAGE plpgsql AS
$$
DECLARE
    st text;
BEGIN
    -- FOR SHARE waits for a concurrent submit, so a source can't slip in after it.
    SELECT status INTO st FROM knowledge_article_versions
    WHERE id = COALESCE(NEW.version_id, OLD.version_id) FOR SHARE;
    IF NOT FOUND THEN
        RETURN COALESCE(NEW, OLD); -- let the foreign key report a missing version
    END IF;
    IF st <> 'draft' THEN
        RAISE EXCEPTION 'sources of a submitted knowledge version are frozen' USING ERRCODE = 'insufficient_privilege';
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER knowledge_version_sources_frozen
    BEFORE INSERT OR UPDATE OR DELETE ON knowledge_version_sources
    FOR EACH ROW EXECUTE FUNCTION knowledge_version_sources_frozen();

-- +goose Down
DROP TABLE knowledge_version_sources;
DROP FUNCTION knowledge_version_sources_frozen();
DROP TABLE knowledge_version_reviews;
DROP TABLE knowledge_article_versions;
DROP FUNCTION knowledge_versions_workflow();
DROP TABLE knowledge_articles;
DROP TABLE knowledge_sources;
DROP FUNCTION knowledge_immutable();
