package ai

type IntentResult struct {
	Kind   string   `json:"kind"`
	Title  string   `json:"title"`
	Goal   string   `json:"goal"`
	Topics []string `json:"topics"`
}

type TaskProposal struct {
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Topic          string   `json:"topic"`
	RequiredSkills []string `json:"required_skills"`
	Dependencies   []int    `json:"dependencies"`
	ExpectedOutput string   `json:"expected_output"`
}

type PlanResult struct {
	Title     string         `json:"title"`
	Rationale string         `json:"rationale"`
	Tasks     []TaskProposal `json:"tasks"`
}

type ClarificationQuestion struct {
	Question      string  `json:"question"`
	Reason        string  `json:"reason"`
	Required      bool    `json:"required"`
	Topic         string  `json:"topic"`
	RelatedTaskID *string `json:"related_task_id,omitempty"`
}

type Gap struct {
	Question  ClarificationQuestion `json:"question"`
	TaskIndex int                   `json:"task_index"`
}

type AssignmentProposal struct {
	TaskID                    string   `json:"task_id"`
	CandidateUserID           *string  `json:"candidate_user_id"`
	CandidateName             string   `json:"candidate_name,omitempty"`
	Evidence                  []string `json:"evidence"`
	Confidence                float64  `json:"confidence"`
	RequiresHumanConfirmation bool     `json:"requires_human_confirmation"`
}

type VerificationResult struct {
	Passed     bool    `json:"passed"`
	Feedback   string  `json:"feedback"`
	Confidence float64 `json:"confidence"`
}

type LearningResult struct {
	Topic           string  `json:"topic"`
	PersonID        *string `json:"person_id"`
	PersonName      string  `json:"person_name,omitempty"`
	ConfidenceDelta float64 `json:"confidence_delta"`
	Summary         string  `json:"summary"`
}

const InitialLearnedConfidence = 0.55

const (
	IntentKindReport    = "create_report"
	IntentKindGeneral   = "general_task"
	IntentKindSmallTalk = "smalltalk"
)

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
