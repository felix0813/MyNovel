CREATE TYPE novel_status AS ENUM ('unread', 'reading', 'finished');

CREATE TABLE IF NOT EXISTS novels (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    platform TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    file_path TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    status novel_status NOT NULL DEFAULT 'unread',
    rating INTEGER NOT NULL DEFAULT 0 CHECK (rating >= 0 AND rating <= 10),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_novels_name ON novels USING gin (to_tsvector('simple', name));
CREATE INDEX IF NOT EXISTS idx_novels_status ON novels(status);

CREATE OR REPLACE VIEW v_novel_stats AS
SELECT
    status,
    COUNT(*) AS total,
    ROUND(AVG(rating)::numeric, 2) AS avg_rating,
    MAX(updated_at) AS last_updated
FROM novels
GROUP BY status;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_novels_set_updated_at ON novels;
CREATE TRIGGER trg_novels_set_updated_at
BEFORE UPDATE ON novels
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
