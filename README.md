# APA — AI-Powered Assignment

Turn a manager's one-line intent into an **approved, assigned and verified plan** — and learn who owns what in the organization along the way.

```
Manager:  "Prepare the cybersecurity awareness report."
AI:       "I created a proposed workflow."            (intent → context → plan → assignments)
AI:       "I don't know who handles incident
           statistics. Who should I assign it to?"    (clarifying question)
Manager:  "Ali."
System:   Stores organizational knowledge.            (learning)
AI:       "Ali Hassan is the proposed assignee."      (evidence-backed proposal)
Manager:  "Approve."
System:   Creates and assigns tasks.                  (human-in-the-loop gate)
…later…
AI:       Proposes Ali directly:                      (memory!)
          "Ali handles 'cybersecurity' (confidence 0.63, seen 2 times, learned)"
```

That learning loop is the heart of the product.

## Architecture

**The backend is 100% Go.** No Python anywhere.

```
Next.js UI ──REST──▶ Go API (chi)
                        │
                 Application services      ← business rules, validation, events
                   │            │
              Domain model   AI Orchestrator ── LLMProvider interface
             (workflows,        ├─ Intent Agent        ├─ MockProvider
              tasks, questions, ├─ Context Agent       └─ OpenAIProvider
              knowledge,        ├─ Planning Agent
              approvals)        ├─ Assignment Agent
                                ├─ Question Agent
                                ├─ Verification Agent
                                └─ Learning Agent
                        │
                   PostgreSQL (pgx)  +  background worker (Postgres-backed jobs)
```

Key rule enforced everywhere:

```
Agent → Proposal → Application service → Validation → Database
```

AI output never touches the database directly. All structured AI responses are validated before persistence (`ai.TaskProposal`, `AssignmentProposal`, `ClarificationQuestion`, …), and every important action is appended to an audit event log in Postgres (`WORKFLOW_CREATED`, `PLAN_GENERATED`, `QUESTION_ANSWERED`, `ASSIGNMENT_PROPOSED`, `TASK_VERIFIED`, `KNOWLEDGE_LEARNED`, …).

## Stack

| Layer | Choice |
|---|---|
| Language | Go 1.24+ (written on 1.26) |
| HTTP | [chi](https://github.com/go-chi/chi) + stdlib middleware |
| DB | PostgreSQL 17 via [pgx](https://github.com/jackc/pgx/v5) |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) embedded, auto-run at boot |
| Auth | JWT (HS256) + bcrypt |
| Logging | `log/slog` structured JSON |
| Jobs | Postgres-backed queue (`FOR UPDATE SKIP LOCKED`), bounded worker pool |
| AI | `LLMProvider` interface → `MockProvider` (deterministic) / `OpenAIProvider` |
| Frontend | Next.js 15 + TypeScript + Tailwind CSS v4 |

Why plain pgx instead of sqlc: the query surface is small and heavily dynamic (filters built at runtime); hand-written pgx keeps everything type-checked by the compiler without a codegen step. Swapping to sqlc later only touches `internal/repository`.

## Run it

### Full stack with Docker

```bash
cp .env.example .env          # optional; defaults work for local dev
docker compose up --build
```

- UI: http://localhost:3000
- API: http://localhost:8080 (health: `GET /healthz`)
- The app starts **empty**: the first person who signs up becomes the organization's **مدیر** (manager); everyone after joins as member. To load demo data instead, set `SEED_DEMO=true` (accounts `sara@acme.test` etc., password `password123`).

### Local development

```bash
docker compose up -d postgres

# terminal 1
make dev-backend

# terminal 2
cd frontend && npm install && npm run dev   # http://localhost:3000
```

To use a real LLM instead of the deterministic mock:

```bash
LLM_PROVIDER=openai LLM_API_KEY=sk-... make dev-backend
```

Everything else (orchestration, validation, HITL gates, learning) is provider-independent.

## The demo, step by step (curl)

```bash
API=http://localhost:8080/api/v1

# 1. Manager signs in
TOKEN=$(curl -s -X POST $API/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"sara@acme.test","password":"password123"}' | jq -r .data.token)

# 2. Manager states an intent → proposed workflow + clarifying question
curl -s -X POST $API/ai/chat -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"message":"Prepare the cybersecurity awareness report."}' | jq .data.text
# → I created a proposed workflow ... I don't know who handles cybersecurity. Who should I assign it to?

QID=<questionId from response>

# 3. Manager answers → knowledge stored, assignment re-proposed with evidence
curl -s -X POST $API/questions/$QID/answer -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"answer":"Ali handles this."}' | jq .data

# 4. Approve → tasks created & assigned
WFID=<workflowId from step 2>
curl -s -X POST $API/workflows/$WFID/approve -H "Authorization: Bearer $TOKEN" | jq .data.message

# 5. Ali starts and completes his task; the async worker verifies it
ALI=$(curl -s -X POST $API/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"ali@acme.test","password":"password123"}' | jq -r .data.token)
TASKID=<task id assigned to Ali>
curl -s -X PATCH $API/tasks/$TASKID -H "Authorization: Bearer $ALI" \
  -H 'Content-Type: application/json' -d '{"action":"start"}'
curl -s -X PATCH $API/tasks/$TASKID -H "Authorization: Bearer $ALI" \
  -H 'Content-Type: application/json' \
  -d '{"action":"complete","notes":"Compiled six months of phishing click rates and incident counts from the SIEM."}'

# 6. Repeat step 2 later → no question asked; Ali proposed from learned knowledge
```

Or just use the Assistant page in the UI — the whole flow works conversationally ("approve" included).

## API

All endpoints under `/api/v1`, JSON envelope `{"data": ...}` / `{"error": {code, message}}`. Errors map to proper status codes (`404 not_found`, `403 forbidden`, `409 invalid_state`, `422 insufficient_data/validation_error`, `502 ai_output_invalid`).

| Method | Path | Purpose |
|---|---|---|
| POST | `/auth/login` | Issue JWT |
| POST | `/auth/register` | Create account (joins the org as member) and issue JWT |
| GET | `/auth/me` | Current user |
| GET/POST | `/workflows` | List / create from intent (runs orchestrator) |
| GET | `/workflows/{id}` | Workflow + tasks + questions |
| POST | `/workflows/{id}/approve` `/reject` | Human decision gate (manager only) |
| GET | `/tasks?mine=true&status=` | Task list |
| GET | `/tasks/{id}` | Task detail |
| PATCH | `/tasks/{id}` | `{action: start \| complete \| resume}` |
| POST | `/tasks/{id}/assign` | Manual override (manager only) |
| GET | `/questions?status=&workflowId=` | Clarification questions |
| POST | `/questions/{id}/answer` | Answer → triggers learning + reassignment |
| GET | `/knowledge`, `/knowledge/people`, `/knowledge/facts` | Organizational memory |
| POST | `/knowledge` | Manual fact (manager only) |
| GET | `/employees` | Team directory |
| POST | `/employees` | Create employee with role + skills (manager only) |
| PATCH | `/employees/{id}` | Change role / skills (manager only) |
| POST | `/ai/chat` | Conversational entry point |
| GET | `/events` | Audit trail (filterable by entity) |
| GET | `/healthz` | Liveness + DB ping |

## Project layout

```
backend/
├── cmd/server/main.go            wiring: config→db→repos→bus→agents→services→api→worker
├── internal/
│   ├── api/                      chi router, JWT middleware, request logging, CORS, handlers
│   ├── application/              workflow/task/question/knowledge/assignment/approval/chat services,
│   │                             event bus, repository ports (interfaces)
│   ├── ai/                       provider.go (LLMProvider), mock.go, openai.go,
│   │                             intent/context/planner/questions/assignment/verification/learning agents,
│   │                             orchestrator.go
│   ├── domain/                   pure entities & rules: workflow, task, question, knowledge, approval, user
│   ├── repository/               pgx implementations of ports
│   ├── infrastructure/           database (pool+migrations), llm client, logging
│   ├── worker/                   Postgres-backed job runner (SKIP LOCKED, bounded concurrency)
│   ├── seed/                     idempotent demo org + users + seeded fact
│   └── config/
├── migrations/0001_init.up.sql   schema
frontend/
└── src/app/(app)/                chat, workflows (+detail), tasks, knowledge pages
docker-compose.yml                postgres + backend + frontend
```

Dependency direction is strictly `HTTP → Application → Domain ← Infrastructure`; domain imports nothing internal, services depend on interfaces defined in `internal/application/ports.go`.

## Concurrency notes

- Intent analysis and org-context gathering run **in parallel** (`errgroup`) per planning request.
- Per-task assignment scoring runs under a bounded errgroup (max 8).
- The worker claims jobs with `FOR UPDATE SKIP LOCKED` and processes them with 4 goroutines max.
- No unbounded goroutines; every goroutine's lifetime is tied to a `context.Context`.
- A data race in the first draft of the parallel scorer was caught live and fixed by using index-keyed slices instead of concurrent map writes.

## Design decisions worth knowing

- **Human-in-the-loop by policy**: assignment proposals always carry `requiresHumanConfirmation=true`; approval requires all required questions answered; blocked verifications need manager guidance to resume.
- **Confidence model**: topic ownership facts start at 0.55 when a manager names a person, +0.08 each time related work is verified delivered, clamped to 0.95. Questions stop being asked once ownership confidence ≥ 0.35.
- **Auditability**: every state change emits a typed event persisted in Postgres (`events` table), exposed via `GET /events`.
- **Graceful degradation**: if the LLM errors mid-plan, the API returns a clean `502 ai_output_invalid` — never raw internals; secrets never leave the process.
