package workflowsvc

import (
	"github.com/apa/backend/internal/domain/question"
	"github.com/apa/backend/internal/domain/task"
	"github.com/apa/backend/internal/domain/workflow"
)

type View struct {
	Workflow       *workflow.Workflow   `json:"workflow"`
	Tasks          []*task.Task         `json:"tasks"`
	Questions      []*question.Question `json:"questions"`
	ProposedOwners []string             `json:"proposedOwners,omitempty"`
}

func (v *View) OpenQuestions() []*question.Question {
	open := []*question.Question{}
	for _, q := range v.Questions {
		if q.Status == question.StatusOpen {
			open = append(open, q)
		}
	}
	return open
}
