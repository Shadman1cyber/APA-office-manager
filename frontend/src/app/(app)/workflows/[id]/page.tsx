"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import { getCachedUser } from "@/lib/auth";
import type { Task, WorkflowView } from "@/lib/types";
import { Badge, Button, Card, Confidence, Input, Spinner } from "@/components/ui";

export default function WorkflowDetailPage() {
  const params = useParams<{ id: string }>();
  const [view, setView] = useState<WorkflowView | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    api
      .workflow(params.id)
      .then(setView)
      .catch((e) => setError(e.message));
  }, [params.id]);

  useEffect(load, [load]);

  async function decide(action: "approve" | "reject") {
    setBusy(true);
    setError(null);
    try {
      if (action === "approve") await api.approveWorkflow(params.id);
      else await api.rejectWorkflow(params.id);
      load();
    } catch (e: any) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }

  async function answerQuestion(questionId: string, answer: string) {
    setBusy(true);
    setError(null);
    try {
      await api.answerQuestion(questionId, answer);
      load();
    } catch (e: any) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }

  if (!view && !error) return <div className="p-10"><Spinner /></div>;
  if (!view) return <p className="p-10 text-sm text-red-600">{error}</p>;

  const wf = view.workflow;
  const openQuestions = view.questions.filter((q) => q.status === "open");
  const manager = getCachedUser()?.role === "manager";
  const canDecide = manager && wf.status === "proposed";

  return (
    <div className="mx-auto max-w-4xl px-8 py-8">
      <div className="mb-6 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold">{wf.title}</h1>
          <p className="mt-1 max-w-2xl text-sm text-slate-500">{wf.intentText}</p>
        </div>
        <Badge status={wf.status} />
      </div>

      {canDecide && (
        <Card className="mb-6 flex items-center justify-between px-5 py-4">
          <p className="text-sm text-slate-600">
            این برنامه در انتظار تصمیم مدیر است.
          </p>
          <div className="flex gap-2">
            <Button variant="secondary" disabled={busy} onClick={() => decide("reject")}>
              رد کردن
            </Button>
            <Button disabled={busy || openQuestions.length > 0} onClick={() => decide("approve")}>
              تأیید و تخصیص
            </Button>
          </div>
        </Card>
      )}

      {error && (
        <p className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
      )}

      {manager && openQuestions.length > 0 && (
        <section className="mb-6 space-y-3">
          <h2 className="text-sm font-semibold text-slate-700">
            سؤال‌های بی‌پاسخ ({openQuestions.length})
          </h2>
          {openQuestions.map((q) => (
            <QuestionRow key={q.id} question={q} onAnswer={answerQuestion} busy={busy} />
          ))}
        </section>
      )}

      <section>
        <h2 className="mb-3 text-sm font-semibold text-slate-700">برنامه</h2>
        <div className="space-y-3">
          {view.tasks.map((t) => (
            <TaskRow key={t.id} task={t} />
          ))}
        </div>
      </section>

      {view.questions.some((q) => q.status === "answered") && (
        <>
          <h2 className="mb-3 mt-8 text-sm font-semibold text-slate-700">سؤال‌های پاسخ داده شده</h2>
          <div className="space-y-2">
            {view.questions
              .filter((q) => q.status === "answered")
              .map((q) => (
                <Card key={q.id} className="px-4 py-3 text-sm">
                  <p className="font-medium">{q.question}</p>
                  <p className="mt-1 text-slate-600">
                    پاسخ: {q.answer}
                    {q.answeredAt && (
                      <span className="ml-2 text-xs text-slate-400">
                        {new Date(q.answeredAt).toLocaleString("fa-IR")}
                      </span>
                    )}
                  </p>
                </Card>
              ))}
          </div>
        </>
      )}
    </div>
  );
}

function QuestionRow({
  question,
  onAnswer,
  busy,
}: {
  question: { id: string; question: string; reason: string; topic: string };
  onAnswer: (id: string, answer: string) => void;
  busy: boolean;
}) {
  const [value, setValue] = useState("");
  return (
    <Card className="border-amber-200 bg-amber-50/50 px-5 py-4">
      <p className="text-sm font-medium">{question.question}</p>
      <p className="mt-1 text-xs text-slate-500">{question.reason}</p>
      <form
        className="mt-3 flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (value.trim()) onAnswer(question.id, value.trim());
        }}
      >
        <Input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder='مثلاً: علی این کار را انجام می‌دهد'
        />
        <Button type="submit" disabled={busy || !value.trim()}>
          ثبت پاسخ
        </Button>
      </form>
    </Card>
  );
}

function TaskRow({ task }: { task: Task }) {
  return (
    <Card className="px-5 py-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-medium">{task.title}</p>
          <p className="mt-0.5 text-xs text-slate-500">{task.description}</p>
          {task.expectedOutput && (
            <p className="mt-1.5 text-xs text-slate-400">
              خروجی مورد انتظار: {task.expectedOutput}
            </p>
          )}
          {task.dependsOn.length > 0 && (
            <p className="mt-1 text-xs text-slate-400">
              به {task.dependsOn.length} وظیفهٔ قبلی وابسته است
            </p>
          )}
        </div>
        <div className="flex shrink-0 flex-col items-end gap-1">
          <Badge status={task.status} />
          <span className="text-xs text-slate-500">
            {task.assigneeName ?? "بدون مسئول"}
          </span>
        </div>
      </div>

      {task.proposal && task.proposal.candidateName && (
        <div className="mt-3 rounded-lg border border-indigo-100 bg-indigo-50/60 px-3 py-2 text-xs">
          <div className="flex items-center gap-2 font-medium text-indigo-800">
            پیشنهاد هوش مصنوعی: {task.proposal.candidateName}
            <Confidence value={task.proposal.confidence} />
          </div>
          <ul className="mt-1 list-inside list-disc text-slate-600">
            {task.proposal.evidence.map((ev, i) => (
              <li key={i}>{ev}</li>
            ))}
          </ul>
        </div>
      )}

      {task.status === "blocked" && (
        <p className="mt-3 rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700">
          بررسی هوش مصنوعی این وظیفه را متوقف کرد. مدیر می‌تواند راهنمایی اضافه کرده و ادامه دهد.
        </p>
      )}
    </Card>
  );
}
