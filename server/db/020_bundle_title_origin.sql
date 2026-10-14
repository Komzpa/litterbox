-- Existing titles have no recorded origin. Preserve them as independent until
-- an evidence-backed owner explicitly identifies a generated seed. Subject/key
-- equality is not provenance: an independent topic may use identical wording.
ALTER TABLE bundles ADD COLUMN IF NOT EXISTS title_origin text NOT NULL DEFAULT 'unknown'
    CHECK (title_origin IN ('unknown', 'explicit', 'seed', 'topic'));
