CREATE TABLE IF NOT EXISTS comments (
    id UUID PRIMARY KEY,
    parent_id UUID REFERENCES comments(id) ON DELETE SET NULL,
    author VARCHAR(100) NOT NULL,
    content TEXT NOT NULL,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    path TEXT NOT NULL,
    depth INT NOT NULL DEFAULT 0 CHECK (depth >= 0),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ
);

CREATE INDEX idx_comments_path ON comments USING btree (path varchar_pattern_ops);

CREATE INDEX idx_comments_fts ON comments USING GIN (
    to_tsvector('english', author || ' ' || content)
);

CREATE INDEX idx_comments_root ON comments (id DESC) WHERE parent_id IS NULL AND is_deleted = FALSE;