const API_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

function token(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem("apa_token");
}

export function setToken(value: string | null) {
  if (typeof window === "undefined") return;
  if (value) window.localStorage.setItem("apa_token", value);
  else window.localStorage.removeItem("apa_token");
}

async function request<T>(
  path: string,
  options: { method?: string; body?: unknown } = {}
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  const tok = token();
  if (tok) headers.Authorization = `Bearer ${tok}`;

  const res = await fetch(`${API_URL}${path}`, {
    method: options.method ?? "GET",
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });

  let payload: any = null;
  try {
    payload = await res.json();
  } catch {}

  if (!res.ok) {
    const message =
      payload?.error?.message ?? `request failed (${res.status})`;
    const staleAuth =
      res.status === 401 ||
      (res.status === 404 && path === "/auth/me");
    if (staleAuth && typeof window !== "undefined" && path !== "/auth/login") {
      setToken(null);
      window.localStorage.removeItem("apa_user");
      window.location.href = "/login";
    }
    throw new ApiError(res.status, payload?.error?.code ?? "error", message);
  }
  return payload?.data as T;
}

export const api = {
  login(email: string, password: string) {
    return request<{ token: string; user: import("./types").User }>("/auth/login", {
      method: "POST",
      body: { email, password },
    });
  },
  me() {
    return request<import("./types").User>("/auth/me");
  },
  chatHistory(limit = 200) {
    return request<import("./types").ChatMessage[]>(`/ai/chat/history?limit=${limit}`);
  },
  chatHistoryDay(day: string) {
    return request<import("./types").ChatMessage[]>(
      `/ai/chat/history?day=${encodeURIComponent(day)}`
    );
  },
  chatDays() {
    return request<import("./types").ChatDaySummary[]>("/ai/chat/history/days");
  },
  chat(message: string) {
    return request<import("./types").ChatReply>("/ai/chat", {
      method: "POST",
      body: { message },
    });
  },
  workflows(limit = 50) {
    return request<import("./types").Workflow[]>(`/workflows?limit=${limit}`);
  },
  workflow(id: string) {
    return request<import("./types").WorkflowView>(`/workflows/${id}`);
  },
  approveWorkflow(id: string) {
    return request<{ message: string }>(`/workflows/${id}/approve`, {
      method: "POST",
      body: {},
    });
  },
  rejectWorkflow(id: string, reason = "") {
    return request<unknown>(`/workflows/${id}/reject`, {
      method: "POST",
      body: { reason },
    });
  },
  tasks(mine: boolean) {
    return request<import("./types").Task[]>(
      `/tasks?mine=${mine ? "true" : "false"}`
    );
  },
  tasksAvailable() {
    return request<import("./types").Task[]>("/tasks?available=true");
  },
  task(id: string) {
    return request<import("./types").Task>(`/tasks/${id}`);
  },
  patchTask(id: string, body: { action: string; notes?: string; guidance?: string }) {
    return request<{ task: import("./types").Task; message: string }>(
      `/tasks/${id}`,
      { method: "PATCH", body }
    );
  },
  assignTask(id: string, userId: string) {
    return request<import("./types").Task>(`/tasks/${id}/assign`, {
      method: "POST",
      body: { userId },
    });
  },
  questions(status?: string) {
    return request<import("./types").Question[]>(
      status ? `/questions?status=${status}` : "/questions"
    );
  },
  answerQuestion(id: string, answer: string) {
    return request<{
      question: import("./types").Question;
      learned?: string;
      task?: import("./types").Task;
    }>(`/questions/${id}/answer`, { method: "POST", body: { answer } });
  },
  employees() {
    return request<import("./types").User[]>("/employees");
  },
  createEmployee(body: {
    name: string;
    email: string;
    password: string;
    role: string;
    skills?: string[];
  }) {
    return request<import("./types").User>("/employees", {
      method: "POST",
      body,
    });
  },
  updateEmployee(id: string, body: { role?: string; skills?: string[] }) {
    return request<import("./types").User>(`/employees/${id}`, {
      method: "PATCH",
      body,
    });
  },
  knowledgeOverview() {
    return request<{ peopleCount: number; factCount: number }>("/knowledge");
  },
  people() {
    return request<import("./types").PersonProfile[]>("/knowledge/people");
  },
  facts() {
    return request<import("./types").Fact[]>("/knowledge/facts");
  },
  events(limit = 100) {
    return request<import("./types").EventRecord[]>(`/events?limit=${limit}`);
  },
};
