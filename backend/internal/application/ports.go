package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain/approval"
	"github.com/apa/backend/internal/domain/document"
	"github.com/apa/backend/internal/domain/knowledge"
	"github.com/apa/backend/internal/domain/organization"
	"github.com/apa/backend/internal/domain/question"
	"github.com/apa/backend/internal/domain/skill"
	"github.com/apa/backend/internal/domain/task"
	"github.com/apa/backend/internal/domain/user"
	"github.com/apa/backend/internal/domain/workflow"
)

type WorkflowRepository interface {
	Create(ctx context.Context, w *workflow.Workflow) error
	Get(ctx context.Context, orgID, id uuid.UUID) (*workflow.Workflow, error)
	List(ctx context.Context, orgID uuid.UUID, limit int) ([]*workflow.Workflow, error)
	ListForUser(ctx context.Context, orgID, userID uuid.UUID) ([]*workflow.Workflow, error)
	UpdateStatusExpected(ctx context.Context, orgID, id uuid.UUID, from, to workflow.Status) error
}

type TaskRepository interface {
	CreateBatch(ctx context.Context, tasks []*task.Task) error
	SetDependencies(ctx context.Context, deps map[uuid.UUID][]uuid.UUID) error
	Get(ctx context.Context, orgID, id uuid.UUID) (*task.Task, error)
	ListByWorkflow(ctx context.Context, workflowID uuid.UUID) ([]*task.Task, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID, assignedTo *uuid.UUID, status string, limit int) ([]*task.Task, error)
	ListAvailable(ctx context.Context, orgID uuid.UUID, limit int) ([]*task.Task, error)
	UpdateStatusExpected(ctx context.Context, id uuid.UUID, from, to task.Status) error
	Assign(ctx context.Context, id uuid.UUID, userID *uuid.UUID, from, to task.Status) error
	SaveProposal(ctx context.Context, id uuid.UUID, p *task.Proposal) error
	SetCompletionNotes(ctx context.Context, id uuid.UUID, notes string) error
	SetDeadline(ctx context.Context, orgID, id uuid.UUID, deadline *time.Time) error
	SetVerified(ctx context.Context, id uuid.UUID, at time.Time) error
	BlockTask(ctx context.Context, id uuid.UUID, from task.Status) error
	WorkflowProgress(ctx context.Context, workflowID uuid.UUID) (total int, verified int, err error)
	UserInvolvedInWorkflow(ctx context.Context, orgID, workflowID, userID uuid.UUID) (bool, error)
}

type QuestionRepository interface {
	Create(ctx context.Context, q *question.Question) error
	Get(ctx context.Context, orgID, id uuid.UUID) (*question.Question, error)
	ListByWorkflow(ctx context.Context, workflowID uuid.UUID) ([]*question.Question, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID, status string, workflowID *uuid.UUID) ([]*question.Question, error)
	ListOpenRequired(ctx context.Context, workflowID uuid.UUID) ([]*question.Question, error)
	PersistAnswer(ctx context.Context, q *question.Question) error
}

type ApprovalRepository interface {
	Create(ctx context.Context, a *approval.Approval) error
	LatestForWorkflow(ctx context.Context, workflowID uuid.UUID, typ approval.Type) (*approval.Approval, error)
	Decide(ctx context.Context, id uuid.UUID, status approval.Status, decidedBy uuid.UUID, at time.Time) error
}

type UserRepository interface {
	Create(ctx context.Context, u *user.User, passwordHash string) error
	Get(ctx context.Context, id uuid.UUID) (*user.User, error)
	GetByEmail(ctx context.Context, email string) (*user.User, string, error)
	List(ctx context.Context, orgID uuid.UUID) ([]user.User, error)
	UpdateRoleSkills(ctx context.Context, id uuid.UUID, role *user.Role, skills []string) error
	Count(ctx context.Context) (int, error)
}

type KnowledgeRepository interface {
	UpsertFact(ctx context.Context, f *knowledge.Fact) error
	FindFacts(ctx context.Context, orgID uuid.UUID, kind knowledge.FactKind, subjects []string) ([]knowledge.Fact, error)
	FindFact(ctx context.Context, orgID uuid.UUID, kind knowledge.FactKind, subject string, personID uuid.UUID) (*knowledge.Fact, error)
	ListAllFacts(ctx context.Context, orgID uuid.UUID) ([]knowledge.Fact, error)
	PeopleProfiles(ctx context.Context, orgID uuid.UUID) ([]knowledge.PersonProfile, error)
	CountFacts(ctx context.Context, orgID uuid.UUID) (int, error)
}

type SkillRepository interface {
	Create(ctx context.Context, sk *skill.Skill) error
	List(ctx context.Context, orgID uuid.UUID) ([]skill.Skill, error)
	Count(ctx context.Context, orgID uuid.UUID) (int, error)
	ExistsByNames(ctx context.Context, orgID uuid.UUID, names []string) (map[string]bool, error)
}

type DocumentRepository interface {
	Create(ctx context.Context, d *document.Document) error
	Get(ctx context.Context, orgID, id uuid.UUID) (*document.Document, error)
	UpdateResult(ctx context.Context, id uuid.UUID, title, body string, status document.Status) error
	ListByOrg(ctx context.Context, orgID uuid.UUID, limit int) ([]*document.Document, error)
	ListByAuthor(ctx context.Context, orgID, authorID uuid.UUID, limit int) ([]*document.Document, error)
}

type ChatRepository interface {
	Append(ctx context.Context, m *ChatMessage) error
	ListByUser(ctx context.Context, orgID, userID uuid.UUID, limit int) ([]*ChatMessage, error)
	ListByUserOnDay(ctx context.Context, orgID, userID uuid.UUID, dayStart, dayEnd time.Time) ([]*ChatMessage, error)
	ListDays(ctx context.Context, orgID, userID uuid.UUID) ([]*ChatDaySummary, error)
}

type OrganizationRepository interface {
	Create(ctx context.Context, name string) (*organization.Organization, error)
	First(ctx context.Context) (*organization.Organization, error)
	Count(ctx context.Context) (int, error)
}
