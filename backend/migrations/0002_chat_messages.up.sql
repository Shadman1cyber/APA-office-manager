CREATE TABLE chat_messages (
    id BIGSERIAL PRIMARY KEY,
    org_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id),
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    text TEXT NOT NULL,
    action TEXT NOT NULL DEFAULT '',
    workflow_id UUID,
    question_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_chat_messages_user ON chat_messages(user_id, id DESC);
