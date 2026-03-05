CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE conversations (
    id UUID PRIMARY KEY,
    message_count BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE messages (
    id UUID PRIMARY KEY,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role SMALLINT NOT NULL,
    kind TEXT NOT NULL,
    body_text TEXT,
    tool_name TEXT,
    tool_call_id TEXT,
    tool_text_output TEXT,
    tool_image_external_url TEXT,
    tool_image_mime_type TEXT,
    tool_image_detail SMALLINT,
    tool_image_width_px INTEGER,
    tool_image_height_px INTEGER,
    tool_image_sha256 TEXT,
    tool_metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE conversation_items (
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    position BIGSERIAL NOT NULL,
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    PRIMARY KEY (conversation_id, position),
    UNIQUE (conversation_id, message_id)
);

CREATE TABLE snapshots (
    id UUID PRIMARY KEY,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE snapshot_items (
    snapshot_id UUID NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    order_idx INTEGER NOT NULL,
    message_id UUID NOT NULL,
    role SMALLINT NOT NULL,
    kind TEXT NOT NULL,
    body_text TEXT,
    tool_name TEXT,
    tool_call_id TEXT,
    tool_text_output TEXT,
    tool_image_external_url TEXT,
    tool_image_mime_type TEXT,
    tool_image_detail SMALLINT,
    tool_image_width_px INTEGER,
    tool_image_height_px INTEGER,
    tool_image_sha256 TEXT,
    tool_metadata JSONB,
    body_sha256 TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (snapshot_id, order_idx)
);

CREATE INDEX idx_messages_conversation_created_at ON messages (conversation_id, created_at);
CREATE INDEX idx_conversation_items_message ON conversation_items (conversation_id, message_id);
CREATE INDEX idx_snapshot_items_snapshot ON snapshot_items (snapshot_id, order_idx);
