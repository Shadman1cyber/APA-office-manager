package chatsvc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/ai"
	"github.com/apa/backend/internal/application"
	questionsvc "github.com/apa/backend/internal/application/question"
	workflowsvc "github.com/apa/backend/internal/application/workflow"
	"github.com/apa/backend/internal/domain"
	"github.com/apa/backend/internal/domain/user"
)

type Reply struct {
	Text       string            `json:"text"`
	Action     string            `json:"action"`
	WorkflowID *uuid.UUID        `json:"workflowId,omitempty"`
	QuestionID *uuid.UUID        `json:"questionId,omitempty"`
	Workflow   *workflowsvc.View `json:"workflow,omitempty"`
}

type session struct {
	lastWorkflowID  *uuid.UUID
	pendingQuestion *uuid.UUID
}

type Service struct {
	workflows *workflowsvc.Service
	questions *questionsvc.Service
	chat      application.ChatRepository

	mu       sync.Mutex
	sessions map[uuid.UUID]*session
}

func NewService(workflows *workflowsvc.Service, questions *questionsvc.Service, chat application.ChatRepository) *Service {
	return &Service{
		workflows: workflows,
		questions: questions,
		chat:      chat,
		sessions:  make(map[uuid.UUID]*session),
	}
}

func (s *Service) Handle(ctx context.Context, actor *user.User, message string, deadline *time.Time) (*Reply, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return &Reply{Text: "چه کاری لازم دارید؟ مثلاً بنویسید: «تهیه گزارش آگاهی سایبری.»", Action: "info"}, nil
	}

	lower := strings.ToLower(message)

	reply, err := s.dispatch(ctx, actor, lower, message, deadline)
	if err != nil {
		return nil, err
	}
	s.record(ctx, actor, message, reply)
	return reply, nil
}

func (s *Service) dispatch(ctx context.Context, actor *user.User, lower, message string, deadline *time.Time) (*Reply, error) {
	switch {
	case containsAny(lower, "approve", "confirmed", "looks good", "تأیید", "تایید", "confirm"):
		sess := s.sessionFor(ctx, actor)
		if sess.lastWorkflowID == nil {
			return &Reply{Text: "فعلاً چیزی برای تأیید نیست. اول بگویید چه کاری نیاز دارید.", Action: "info"}, nil
		}
		view, assigned, err := s.workflows.Approve(ctx, actor, *sess.lastWorkflowID)
		if err != nil {
			if strings.Contains(err.Error(), "answer") || strings.Contains(err.Error(), "question") {
				return &Reply{
					Text:       "قبل از تأیید این برنامه، چند سؤال هنوز بی‌پاسخ است.",
					Action:     "needs_answers",
					WorkflowID: sess.lastWorkflowID,
				}, nil
			}
			return nil, err
		}
		text := fmt.Sprintf("تأیید شد؛ %d وظیفه ساخته شد.", len(view.Tasks))
		if len(assigned) > 0 {
			text += " تخصیص‌ها: " + strings.Join(assigned, "؛ ") + "."
		}
		text += " اعضای تیم می‌توانند کارشان را شروع کنند."
		wfID := view.Workflow.ID
		return &Reply{Text: text, Action: "approved", WorkflowID: &wfID, Workflow: view}, nil

	case containsAny(lower, "reject", "discard", "cancel the workflow", "رد", "لغو", "cancel"):
		sess := s.sessionFor(ctx, actor)
		if sess.lastWorkflowID == nil {
			return &Reply{Text: "گردش‌کار فعالی برای رد کردن وجود ندارد. اول درخواست خود را بگویید.", Action: "info"}, nil
		}
		view, err := s.workflows.Reject(ctx, actor, *sess.lastWorkflowID, message)
		if err != nil {
			return nil, err
		}
		sess.pendingQuestion = nil
		wfID := view.Workflow.ID
		return &Reply{Text: "گردش‌کار رد شد.", Action: "rejected", WorkflowID: &wfID, Workflow: view}, nil

	default:
		sess := s.sessionFor(ctx, actor)

		// pending question from previous workflow
		if sess.pendingQuestion != nil {
			pendingID := *sess.pendingQuestion
			q, err := s.questions.Get(ctx, actor.OrgID, pendingID)
			if err != nil || q.Status == "answered" {
				sess.pendingQuestion = nil
			} else {
				return s.handleAnswer(ctx, actor, sess, pendingID, message, deadline)
			}
		}

		// smalltalk → conversational reply (no workflow creation)
		intent := ai.ClassifyIntent(message)
		if intent == ai.IntentKindSmallTalk {
			return s.handleSmallTalk(ctx, actor, sess)
		}

		return s.handleNewIntent(ctx, actor, message, deadline)
	}
}

func (s *Service) record(ctx context.Context, actor *user.User, incoming string, reply *Reply) {
	if s.chat == nil || reply == nil {
		return
	}
	orgID := actor.OrgID
	userMsg := &application.ChatMessage{
		OrgID: orgID, UserID: actor.ID, Role: application.ChatRoleUser,
		Text: incoming, WorkflowID: reply.WorkflowID,
	}
	assistantMsg := &application.ChatMessage{
		OrgID: orgID, UserID: actor.ID, Role: application.ChatRoleAssistant,
		Text: reply.Text, Action: reply.Action,
		WorkflowID: reply.WorkflowID, QuestionID: reply.QuestionID,
	}
	if err := s.chat.Append(ctx, userMsg); err != nil {
		log.Printf("persist user chat message failed: %v", err)
	}
	if err := s.chat.Append(ctx, assistantMsg); err != nil {
		log.Printf("persist assistant chat message failed: %v", err)
	}
}

func (s *Service) History(ctx context.Context, actor *user.User, limit int) ([]*application.ChatMessage, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	return s.chat.ListByUser(ctx, actor.OrgID, actor.ID, limit)
}

func (s *Service) HistoryOnDay(ctx context.Context, actor *user.User, day string) ([]*application.ChatMessage, error) {
	start, end, err := tehranDayRange(day)
	if err != nil {
		return nil, err
	}
	return s.chat.ListByUserOnDay(ctx, actor.OrgID, actor.ID, start, end)
}

func (s *Service) Days(ctx context.Context, actor *user.User) ([]*application.ChatDaySummary, error) {
	return s.chat.ListDays(ctx, actor.OrgID, actor.ID)
}

func tehranDayRange(day string) (time.Time, time.Time, error) {
	tz := time.FixedZone("Asia/Tehran", 3*3600+1800)
	parsed, err := time.ParseInLocation("2006-01-02", day, tz)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: تاریخ نامعتبر است", domain.ErrInvalidState)
	}
	return parsed, parsed.Add(24 * time.Hour), nil
}

func (s *Service) handleAnswer(ctx context.Context, actor *user.User, sess *session, questionID uuid.UUID, message string, deadline *time.Time) (*Reply, error) {
	result, err := s.questions.Answer(ctx, actor, questionID, message)
	if err != nil {
		if strings.Contains(err.Error(), "already answered") {
			sess.pendingQuestion = nil
			return s.handleNewIntent(ctx, actor, message, deadline)
		}
		return nil, err
	}
	sess.pendingQuestion = nil

	reply := &Reply{Action: "answered"}
	if sess.lastWorkflowID != nil {
		reply.WorkflowID = sess.lastWorkflowID
	}

	var b strings.Builder
	if result.Learned != "" {
		b.WriteString(result.Learned)
		b.WriteString("\n\n")
	} else if result.Question.Topic != "" {
		b.WriteString("متشکرم! اما نتوانستم این پاسخ را به یکی از اعضای تیم نگاشت کنم.\n\n")
	}

	if t := result.Task; t != nil && t.Proposal != nil && t.Proposal.CandidateUserID != nil {
		fmt.Fprintf(&b, "%s به‌عنوان مسئول «%s» پیشنهاد شده است (اطمینان %.0f%%).\n",
			t.Proposal.CandidateName, t.Title, t.Proposal.Confidence*100)
		b.WriteString("برای تأیید و ساخت وظایف بگویید «تأیید».")
	} else if sess.lastWorkflowID != nil {
		open, oerr := s.questions.OpenForWorkflow(ctx, actor.OrgID, *sess.lastWorkflowID)
		if oerr == nil && len(open) > 0 {
			next := open[0]
			sess.pendingQuestion = &next.ID
			reply.QuestionID = &next.ID
			b.WriteString(next.Text)
		} else {
			b.WriteString("برای تأیید برنامه بگویید «تأیید».")
		}
	} else {
		b.WriteString("برای تأیید برنامه بگویید «تأیید».")
	}

	reply.Text = b.String()
	return reply, nil
}

func (s *Service) handleSmallTalk(ctx context.Context, actor *user.User, sess *session) (*Reply, error) {
	// If there's a last workflow with pending questions, mention it
	if sess.lastWorkflowID != nil {
		open, err := s.questions.OpenForWorkflow(ctx, actor.OrgID, *sess.lastWorkflowID)
		if err == nil && len(open) > 0 {
			next := open[0]
			sess.pendingQuestion = &next.ID
			return &Reply{
				Text:       next.Text,
				Action:     "info",
				QuestionID: &next.ID,
			}, nil
		}
	}

	replies := []string{
		"سلام! خوش آمدید. کاری هست که بتوانم کمکتان کنم؟ مثلاً بنویسید: «تهیه گزارش آگاهی سایبری.»",
		"خوش آمدید! اگر کاری پیش آمد، بگویید تا برنامه‌اش را آماده کنم.",
		"خوش آمدید! برای شروع یک کار جدید، کافیست بنویسید چه می‌خواهید انجام دهید.",
		"سلام! من اینجا هستم تا درخواست‌هایتان را به وظایف تبدیل کنم. هر وقت آماده بودید، بگویید.",
		"خوش آمدید! می‌توانم برایتان گردش‌کار بسازم. فقط کافیست بگویید چه کاری نیاز دارید.",
	}
	reply := replies[len(actor.Name)%len(replies)]
	return &Reply{Text: reply, Action: "info"}, nil
}

func (s *Service) handleNewIntent(ctx context.Context, actor *user.User, message string, deadline *time.Time) (*Reply, error) {
	view, err := s.workflows.Create(ctx, actor, message, deadline)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return &Reply{
				Text:   "ثبت درخواست جدید فقط برای مدیران فعال است. اگر کاری به شما تخصیص یافته، از صفحهٔ وظایف پیگیری کنید.",
				Action: "info",
			}, nil
		}
		return nil, err
	}

	sess := s.sessionFor(ctx, actor)
	wfID := view.Workflow.ID
	sess.lastWorkflowID = &wfID
	sess.pendingQuestion = nil

	text := fmt.Sprintf("یک گردش‌کار پیشنهادی با عنوان «%s» و %d وظیفه ساختم.", view.Workflow.Title, len(view.Tasks))
	for _, owner := range view.ProposedOwners {
		text += "\n• پیشنهاد: " + owner
	}

	reply := &Reply{Action: "created", WorkflowID: &wfID, Workflow: view}

	open := view.OpenQuestions()
	if len(open) > 0 {
		first := open[0]
		sess.pendingQuestion = &first.ID
		reply.QuestionID = &first.ID
		text += "\n\n" + first.Text
		if first.Reason != "" {
			text += " (" + first.Reason + ")"
		}
	} else {
		text += "\nبرای تأیید برنامه و تخصیص کارها بگویید «تأیید»."
	}
	reply.Text = text
	return reply, nil
}

func (s *Service) sessionFor(ctx context.Context, actor *user.User) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.sessions[actor.ID]
	if sess != nil {
		return sess
	}
	sess = &session{}
	if len(s.sessions) > 4096 {
		s.sessions = make(map[uuid.UUID]*session)
	}
	s.sessions[actor.ID] = sess

	msgs, err := s.chat.ListByUser(ctx, actor.OrgID, actor.ID, 50)
	if err != nil || len(msgs) == 0 {
		return sess
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].WorkflowID != nil {
			wfID := *msgs[i].WorkflowID
			sess.lastWorkflowID = &wfID
			break
		}
	}
	if sess.lastWorkflowID == nil {
		return sess
	}
	if open, oerr := s.questions.OpenForWorkflow(ctx, actor.OrgID, *sess.lastWorkflowID); oerr == nil && len(open) > 0 {
		qid := open[0].ID
		sess.pendingQuestion = &qid
	}
	return sess
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}
