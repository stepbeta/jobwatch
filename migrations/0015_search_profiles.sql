-- 0015_search_profiles: user search profiles replace the hardcoded
-- classifier decisions.
--
-- Offer facts (title, location, description, remote policy) stay on
-- job_offers. User preferences move to search_profiles and its two child
-- tables. Title and location affinity are computed at read time from the
-- active profile; no affinity score is stored. The legacy
-- title_confidence, location_confidence and match_reasons_json columns
-- are left in place, unused and deprecated (additive migration, no table
-- rebuild).
--
-- The seed is a one-to-one copy of the term tables that lived in
-- internal/classify before this change: every title, location and
-- description term keeps its exact weight. `bucket` is a display label
-- for the five-bucket scale; `weight` is the value the scorer uses.
-- Title scale: strong positive +60, mild positive +25/+20/+15, mild
-- negative -20, strong negative -120. Location/description scale: strong
-- positive +35, mild positive +25/+20/+15/+10/+5, mild negative
-- -20/-15, strong negative -35.
--
-- The seed accepts the `remote` and `unknown` policies and rejects
-- `hybrid` and `onsite`. A future product can replace the seed with a
-- from-scratch migration; this one never touches offer rows.
--
-- Rollback: the three tables are additive and the previous binary
-- ignores them. The legacy columns are untouched, so restoring the
-- pre-migration backup is the only step needed.

CREATE TABLE search_profiles (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    name           TEXT NOT NULL,
    title_label    TEXT NOT NULL DEFAULT '',
    location_label TEXT NOT NULL DEFAULT '',
    bucket_scale   TEXT NOT NULL DEFAULT 'five',
    is_default     INTEGER NOT NULL DEFAULT 0,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL
);

-- At most one default profile.
CREATE UNIQUE INDEX idx_search_profiles_default
    ON search_profiles(is_default) WHERE is_default = 1;

CREATE TABLE search_profile_terms (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id INTEGER NOT NULL REFERENCES search_profiles(id) ON DELETE CASCADE,
    affinity   TEXT NOT NULL,
    bucket     TEXT NOT NULL,
    term       TEXT NOT NULL,
    weight     INTEGER NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    UNIQUE (profile_id, affinity, term)
);

CREATE INDEX idx_search_profile_terms_profile
    ON search_profile_terms(profile_id, affinity, sort_order);

CREATE TABLE search_profile_remote (
    profile_id INTEGER NOT NULL REFERENCES search_profiles(id) ON DELETE CASCADE,
    policy     TEXT NOT NULL,
    accepted   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (profile_id, policy)
);

INSERT INTO search_profiles(
    name, title_label, location_label, bucket_scale, is_default, created_at, updated_at)
VALUES (
    'Default', 'Platform Engineer / SRE', 'France / full remote', 'five', 1,
    strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));

WITH seed(affinity, bucket, term, weight, sort_order) AS (
    VALUES
    ('title', 'strong_positive', 'platform engineer', 60, 0),
    ('title', 'strong_positive', 'platform engineering', 60, 1),
    ('title', 'strong_positive', 'platform sre', 60, 2),
    ('title', 'strong_positive', 'site reliability', 60, 3),
    ('title', 'strong_positive', 'sre', 60, 4),
    ('title', 'strong_positive', 'devops', 60, 5),
    ('title', 'strong_positive', 'infrastructure engineer', 60, 6),
    ('title', 'strong_positive', 'cloud engineer', 60, 7),
    ('title', 'mild_positive', 'kubernetes', 25, 8),
    ('title', 'mild_positive', 'reliability', 25, 9),
    ('title', 'mild_positive', 'backend infrastructure', 25, 10),
    ('title', 'mild_positive', 'platform', 25, 11),
    ('title', 'mild_positive', 'infrastructure', 25, 12),
    ('title', 'mild_positive', 'cloud', 25, 13),
    ('title', 'mild_positive', 'terraform', 20, 14),
    ('title', 'mild_positive', 'observability', 20, 15),
    ('title', 'mild_positive', 'distributed systems', 20, 16),
    ('title', 'mild_positive', 'linux', 15, 17),
    ('title', 'strong_negative', 'marketing', -120, 18),
    ('title', 'strong_negative', 'sales', -120, 19),
    ('title', 'strong_negative', 'recruiter', -120, 20),
    ('title', 'strong_negative', 'internship', -120, 21),
    ('title', 'strong_negative', 'account executive', -120, 22),
    ('title', 'strong_negative', 'talent acquisition', -120, 23),
    ('title', 'strong_negative', 'business development', -120, 24),
    ('title', 'strong_negative', 'intern', -120, 25),
    ('title', 'strong_negative', 'trainee', -120, 26),
    ('title', 'strong_negative', 'apprentice', -120, 27),
    ('title', 'strong_negative', 'stage', -120, 28),
    ('title', 'strong_negative', 'alternance', -120, 29),
    ('title', 'mild_negative', 'manager', -20, 30),
    ('title', 'mild_negative', 'director', -20, 31),
    ('title', 'mild_negative', 'head of', -20, 32),
    ('title', 'mild_negative', 'vp', -20, 33),
    ('title', 'mild_negative', 'chief', -20, 34),
    ('location', 'strong_positive', 'france', 35, 0),
    ('location', 'mild_positive', 'paris', 25, 1),
    ('location', 'mild_positive', 'lyon', 25, 2),
    ('location', 'mild_positive', 'nantes', 25, 3),
    ('location', 'mild_positive', 'bordeaux', 25, 4),
    ('location', 'mild_positive', 'lille', 25, 5),
    ('location', 'mild_positive', 'toulouse', 25, 6),
    ('location', 'mild_positive', 'marseille', 25, 7),
    ('location', 'mild_positive', 'rennes', 25, 8),
    ('location', 'mild_positive', 'grenoble', 25, 9),
    ('location', 'mild_positive', 'montpellier', 25, 10),
    ('location', 'mild_positive', 'strasbourg', 25, 11),
    ('location', 'mild_positive', 'nice', 25, 12),
    ('location', 'mild_positive', 'sophia antipolis', 25, 13),
    ('location', 'mild_positive', 'emea', 20, 14),
    ('location', 'mild_positive', 'european union', 15, 15),
    ('location', 'mild_positive', 'europe', 15, 16),
    ('location', 'mild_positive', 'eu', 15, 17),
    ('location', 'mild_positive', 'worldwide', 10, 18),
    ('location', 'mild_positive', 'global', 10, 19),
    ('location', 'mild_positive', 'central european time', 10, 20),
    ('location', 'mild_positive', 'european time zones', 10, 21),
    ('location', 'mild_positive', 'eu time zones', 10, 22),
    ('location', 'mild_positive', 'paris time', 10, 23),
    ('location', 'mild_positive', 'cet', 10, 24),
    ('location', 'mild_positive', 'cest', 10, 25),
    ('location', 'mild_positive', 'utc 1', 10, 26),
    ('location', 'mild_positive', 'utc 2', 10, 27),
    ('location', 'strong_negative', 'united states', -35, 28),
    ('location', 'strong_negative', 'usa', -35, 29),
    ('location', 'strong_negative', 'us', -35, 30),
    ('location', 'strong_negative', 'canada', -35, 31),
    ('location', 'strong_negative', 'new york', -35, 32),
    ('location', 'strong_negative', 'san francisco', -35, 33),
    ('location', 'strong_negative', 'seattle', -35, 34),
    ('location', 'strong_negative', 'austin', -35, 35),
    ('location', 'strong_negative', 'boston', -35, 36),
    ('location', 'strong_negative', 'los angeles', -35, 37),
    ('location', 'strong_negative', 'chicago', -35, 38),
    ('location', 'strong_negative', 'denver', -35, 39),
    ('location', 'strong_negative', 'toronto', -35, 40),
    ('location', 'strong_negative', 'vancouver', -35, 41),
    ('location', 'strong_negative', 'montreal', -35, 42),
    ('location', 'strong_negative', 'india', -35, 43),
    ('location', 'strong_negative', 'bangalore', -35, 44),
    ('location', 'strong_negative', 'bengaluru', -35, 45),
    ('location', 'strong_negative', 'australia', -35, 46),
    ('location', 'strong_negative', 'sydney', -35, 47),
    ('location', 'strong_negative', 'melbourne', -35, 48),
    ('location', 'strong_negative', 'singapore', -35, 49),
    ('location', 'strong_negative', 'japan', -35, 50),
    ('location', 'strong_negative', 'tokyo', -35, 51),
    ('location', 'strong_negative', 'china', -35, 52),
    ('location', 'strong_negative', 'brazil', -35, 53),
    ('location', 'strong_negative', 'mexico', -35, 54),
    ('location', 'strong_negative', 'apac', -35, 55),
    ('location', 'strong_negative', 'latam', -35, 56),
    ('location', 'strong_negative', 'south america', -35, 57),
    ('location', 'mild_negative', 'united kingdom', -20, 58),
    ('location', 'mild_negative', 'uk', -20, 59),
    ('location', 'mild_negative', 'london', -20, 60),
    ('location', 'mild_negative', 'germany', -20, 61),
    ('location', 'mild_negative', 'berlin', -20, 62),
    ('location', 'mild_negative', 'munich', -20, 63),
    ('location', 'mild_negative', 'spain', -15, 64),
    ('location', 'mild_negative', 'madrid', -15, 65),
    ('location', 'mild_negative', 'barcelona', -15, 66),
    ('location', 'mild_negative', 'netherlands', -15, 67),
    ('location', 'mild_negative', 'amsterdam', -15, 68),
    ('location', 'mild_negative', 'belgium', -15, 69),
    ('location', 'mild_negative', 'brussels', -15, 70),
    ('location', 'mild_negative', 'italy', -15, 71),
    ('location', 'mild_negative', 'milan', -15, 72),
    ('location', 'mild_negative', 'portugal', -15, 73),
    ('location', 'mild_negative', 'lisbon', -15, 74),
    ('location', 'mild_negative', 'poland', -15, 75),
    ('location', 'mild_negative', 'warsaw', -15, 76),
    ('location', 'mild_negative', 'switzerland', -15, 77),
    ('location', 'mild_negative', 'zurich', -15, 78),
    ('description', 'mild_positive', 'france', 20, 0),
    ('description', 'mild_positive', 'paris', 15, 1),
    ('description', 'mild_positive', 'emea', 15, 2),
    ('description', 'mild_positive', 'european union', 10, 3),
    ('description', 'mild_positive', 'europe', 10, 4),
    ('description', 'mild_positive', 'worldwide', 5, 5),
    ('description', 'mild_positive', 'global', 5, 6),
    ('description', 'mild_positive', 'central european time', 5, 7),
    ('description', 'mild_positive', 'european time zones', 5, 8),
    ('description', 'mild_positive', 'eu time zones', 5, 9),
    ('description', 'mild_positive', 'paris time', 5, 10),
    ('description', 'mild_positive', 'cet', 5, 11),
    ('description', 'mild_positive', 'cest', 5, 12),
    ('description', 'mild_positive', 'utc 1', 5, 13),
    ('description', 'mild_positive', 'utc 2', 5, 14)
)
INSERT INTO search_profile_terms(profile_id, affinity, bucket, term, weight, sort_order)
SELECT p.id, seed.affinity, seed.bucket, seed.term, seed.weight, seed.sort_order
FROM seed, search_profiles p
WHERE p.is_default = 1;

INSERT INTO search_profile_remote(profile_id, policy, accepted)
SELECT p.id, policy, accepted
FROM search_profiles p,
     (SELECT 'remote' AS policy, 1 AS accepted
      UNION ALL SELECT 'hybrid', 0
      UNION ALL SELECT 'onsite', 0
      UNION ALL SELECT 'unknown', 1)
WHERE p.is_default = 1;
