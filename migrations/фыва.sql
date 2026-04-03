-- Включаем расширение для генерации UUID (если его нет)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Таблица комментариев с Materialized Path
CREATE TABLE IF NOT EXISTS comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id UUID REFERENCES comments(id) ON DELETE SET NULL,
    path TEXT NOT NULL,
    author VARCHAR(100) NOT NULL,
    content TEXT NOT NULL,
    depth INT NOT NULL DEFAULT 0 CHECK (depth >= 0),
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Индекс для быстрого поиска поддеревьев по path (LIKE 'prefix%')
CREATE INDEX idx_comments_path ON comments USING btree (path varchar_pattern_ops);

-- Индекс для полнотекстового поиска по автору и контенту
CREATE INDEX idx_comments_fts ON comments USING GIN (
    to_tsvector('english', author || ' ' || content)
);

-- Индекс для пагинации корней
CREATE INDEX idx_comments_root ON comments (id DESC) WHERE parent_id IS NULL AND is_deleted = FALSE;