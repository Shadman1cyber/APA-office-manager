CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    task_id UUID REFERENCES tasks(id),
    workflow_id UUID,
    author_id UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    source_notes TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'generating' CHECK (status IN ('generating','ready','failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_documents_org ON documents(org_id, created_at DESC);
CREATE INDEX idx_documents_author ON documents(author_id, created_at DESC);
