export type Role = "manager" | "member";

export interface User {
  id: string;
  orgId: string;
  email: string;
  name: string;
  role: Role;
  skills: string[];
  createdAt: string;
}

export type WorkflowStatus =
  | "proposed"
  | "approved"
  | "rejected"
  | "in_progress"
  | "completed"
  | "failed";

export interface Workflow {
  id: string;
  orgId: string;
  createdBy: string;
  title: string;
  intentText: string;
  status: WorkflowStatus;
  createdAt: string;
  updatedAt: string;
}

export type TaskStatus =
  | "proposed"
  | "pending"
  | "assigned"
  | "in_progress"
  | "completed"
  | "verified"
  | "blocked";

export interface Proposal {
  candidateUserId?: string;
  candidateName?: string;
  evidence: string[];
  confidence: number;
  requiresHumanConfirmation: boolean;
}

export interface Task {
  id: string;
  orgId: string;
  workflowId: string;
  position: number;
  title: string;
  description: string;
  topic: string;
  requiredSkills: string[];
  dependsOn: string[];
  expectedOutput: string;
  status: TaskStatus;
  assignedTo?: string;
  assigneeName?: string;
  proposal?: Proposal;
  completedNotes?: string;
  verifiedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface Question {
  id: string;
  orgId: string;
  workflowId: string;
  relatedTaskId?: string;
  topic: string;
  question: string;
  reason: string;
  required: boolean;
  status: "open" | "answered";
  answer?: string;
  answeredBy?: string;
  createdAt: string;
  answeredAt?: string;
}

export interface WorkflowView {
  workflow: Workflow;
  tasks: Task[];
  questions: Question[];
  proposedOwners?: string[];
}

export interface ChatReply {
  text: string;
  action:
    | "created"
    | "answered"
    | "approved"
    | "rejected"
    | "needs_answers"
    | "info";
  workflowId?: string;
  questionId?: string;
  workflow?: WorkflowView;
}

export interface PersonProfile {
  id: string;
  orgId: string;
  email: string;
  name: string;
  role: Role;
  skills: string[];
  createdAt: string;
  ownedTopics: { subject: string; confidence: number; evidenceCount: number }[];
}

export interface Fact {
  id: string;
  orgId: string;
  kind: "topic_owner" | "skill";
  subject: string;
  personId: string;
  personName?: string;
  confidence: number;
  source: "seeded" | "learned";
  evidence: string;
  evidenceCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface ChatDaySummary {
  day: string;
  count: number;
  preview: string;
}

export interface ChatMessage {
  id: number;
  orgId: string;
  userId: string;
  role: "user" | "assistant";
  text: string;
  action?: string;
  workflowId?: string;
  questionId?: string;
  createdAt: string;
}

export interface EventRecord {
  id: number;
  orgId: string;
  type: string;
  entityType: string;
  entityId: string;
  actorId?: string;
  timestamp: string;
  payload: Record<string, unknown>;
}
