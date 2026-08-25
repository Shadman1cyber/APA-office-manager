BEGIN;

CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('manager', 'member')),
    password_hash TEXT NOT NULL,
    skills TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_org ON users(org_id);

CREATE TABLE workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    created_by UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    intent_text TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('proposed', 'approved', 'rejected', 'in_progress', 'completed', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflows_org ON workflows(org_id, created_at DESC);

CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    position INT NOT NULL DEFAULT 0,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    topic TEXT NOT NULL DEFAULT '',
    required_skills TEXT[] NOT NULL DEFAULT '{}',
    depends_on UUID[] NOT NULL DEFAULT '{}',
    expected_output TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('proposed', 'pending', 'assigned', 'in_progress', 'completed', 'verified', 'blocked')),
    assigned_to UUID REFERENCES users(id),
    proposal JSONB,
    completed_notes TEXT NOT NULL DEFAULT '',
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tasks_workflow ON tasks(workflow_id, position);
CREATE INDEX idx_tasks_assignee ON tasks(assigned_to) WHERE assigned_to IS NOT NULL;

CREATE TABLE questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    task_index INT NOT NULL DEFAULT -1,
    related_task_id UUID REFERENCES tasks(id),
    topic TEXT NOT NULL DEFAULT '',
    question TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    required BOOLEAN NOT NULL DEFAULT TRUE,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'answered')),
    answer TEXT NOT NULL DEFAULT '',
    answered_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    answered_at TIMESTAMPTZ
);

CREATE INDEX idx_questions_workflow ON questions(workflow_id);
CREATE INDEX idx_questions_open ON questions(status) WHERE status = 'open';

CREATE TABLE knowledge_facts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    kind TEXT NOT NULL CHECK (kind IN ('topic_owner', 'skill')),
    subject TEXT NOT NULL,
    person_id UUID NOT NULL REFERENCES users(id),
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    source TEXT NOT NULL CHECK (source IN ('seeded', 'learned')),
    evidence TEXT NOT NULL DEFAULT '',
    evidence_count INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (kind, subject, person_id)
);

CREATE INDEX idx_facts_subject ON knowledge_facts(org_id, kind, subject);

CREATE TABLE approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('plan')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
    payload JSONB NOT NULL DEFAULT '{}',
    decided_by UUID REFERENCES users(id),
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_approvals_workflow ON approvals(workflow_id);

CREATE TABLE events (
    id BIGSERIAL PRIMARY KEY,
    org_id UUID NOT NULL,
    type TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    actor_id UUID,
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_events_org_time ON events(org_id, created_at DESC);
CREATE INDEX idx_events_entity ON events(entity_type, entity_id);

CREATE TABLE jobs (
    id BIGSERIAL PRIMARY KEY,
    type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    run_after TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_jobs_pending ON jobs(run_after) WHERE status = 'pending';

COMMIT;
