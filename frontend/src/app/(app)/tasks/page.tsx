"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { getCachedUser } from "@/lib/auth";
import type { Task } from "@/lib/types";
import { Badge, Button, Card, Input, Spinner } from "@/components/ui";
import { DeadlineChip } from "@/components/deadline";

type Tab = "mine" | "pool" | "all";

export default function TasksPage() {
  const manager = getCachedUser()?.role === "manager";
  const [tab, setTab] = useState<Tab>("mine");
  const [tasks, setTasks] = useState<Task[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = useCallback(() => {
    setTasks(null);
    const fetch =
      tab === "pool"
        ? api.tasksAvailable()
        : api.tasks(tab === "mine");
    fetch.then(setTasks).catch((e) => setError(e.message));
  }, [tab]);

  useEffect(load, [load]);

  async function act(task: Task, body: any) {
    setBusyId(task.id);
    setError(null);
    setNotice(null);
    try {
      if (body.action === "__deadline__") {
        const res = await api.setTaskDeadline(task.id, body.notes);
        setNotice(res.message);
        load();
        return;
      }
      if (body.action === "__deadline_clear__") {
        const res = await api.setTaskDeadline(task.id, null);
        setNotice(res.message);
        load();
        return;
      }
      await api.patchTask(task.id, body);
      if (body.action === "claim") {
        setNotice("وظیفه به شما تخصیص یافت؛ در تب «وظایف من» می‌توانید شروع کنید.");
        setTab("mine");
      } else {
        load();
      }
    } catch (e: any) {
      setError(e.message);
    } finally {
      setBusyId(null);
    }
  }

  const tabs: { key: Tab; label: string }[] = [
    { key: "mine", label: "وظایف من" },
    ...(manager ? [{ key: "all" as Tab, label: "همه" }] : []),
    ...(!manager ? [{ key: "pool" as Tab, label: "قابل دریافت" }] : []),
  ];

  return (
    <div className="mx-auto max-w-4xl px-8 py-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">وظایف</h1>
          <p className="mt-1 text-sm text-slate-500">
            {manager
              ? "مسیر هر کار: تخصیص ← شروع ← تکمیل ← بررسی هوش مصنوعی."
              : "وظایف خودتان را انجام دهید یا از وظایف بی‌مسئول، یکی را دریافت کنید."}
          </p>
        </div>
        <div className="flex rounded-lg border border-slate-200 p-1 text-sm">
          {tabs.map((t) => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`rounded-md px-3 py-1.5 transition ${
                tab === t.key
                  ? "bg-indigo-600 font-medium text-white"
                  : "text-slate-600 hover:bg-slate-50"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {error && (
        <p className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
      )}
      {notice && (
        <p className="mb-4 rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-700">{notice}</p>
      )}
      {!tasks && !error && <Spinner />}

      <div className="space-y-3">
        {tab === "pool"
          ? tasks?.map((t) => (
              <Card key={t.id} className="px-5 py-4">
                <PoolTaskItem task={t} busy={busyId === t.id} onClaim={() => act(t, { action: "claim" })} />
              </Card>
            ))
          : tasks?.map((t) => (
              <Card key={t.id} className="px-5 py-4">
                <TaskItem task={t} busy={busyId === t.id} onAct={act} />
              </Card>
            ))}
        {tasks && tasks.length === 0 && (
          <p className="text-sm text-slate-400">
            {tab === "pool"
              ? "فعلاً وظیفهٔ بی‌مسئولی برای دریافت وجود ندارد."
              : "چیزی اینجا نیست."}
          </p>
        )}
      </div>
    </div>
  );
}

function PoolTaskItem({
  task,
  busy,
  onClaim,
}: {
  task: Task;
  busy: boolean;
  onClaim: () => void;
}) {
  return (
    <>
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-medium">{task.title}</p>
          <p className="mt-0.5 text-xs text-slate-500">{task.description}</p>
          <p className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-slate-400">
            {task.topic && <span>موضوع: {task.topic}</span>}
            {task.requiredSkills.length > 0 && (
              <span>مهارت‌های لازم: {task.requiredSkills.join("، ")}</span>
            )}
            {task.expectedOutput && <span>خروجی مورد انتظار: {task.expectedOutput}</span>}
          </p>
        </div>
        <Badge status={task.status} />
      </div>
      <div className="mt-3 border-t border-slate-100 pt-3">
        <Button disabled={busy} onClick={onClaim}>
          {busy ? "در حال ثبت…" : "دریافت این وظیفه"}
        </Button>
      </div>
    </>
  );
}

function TaskItem({
  task,
  busy,
  onAct,
}: {
  task: Task;
  busy: boolean;
  onAct: (task: Task, body: { action: string; notes?: string; guidance?: string }) => void;
}) {
  const [notes, setNotes] = useState("");
  const [guidance, setGuidance] = useState("");
  const [showComplete, setShowComplete] = useState(false);
  const [showResume, setShowResume] = useState(false);

  const canStart = task.status === "assigned";
  const canComplete = task.status === "in_progress";
  const canResume = task.status === "blocked" && getCachedUser()?.role === "manager";

  return (
    <>
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-medium">{task.title}</p>
          <p className="mt-0.5 text-xs text-slate-500">{task.description}</p>
          <DeadlineChip task={task} />
          <p className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-slate-400">
            <span>مسئول: {task.assigneeName ?? "—"}</span>
            {task.topic && <span>موضوع: {task.topic}</span>}
            {task.requiredSkills.length > 0 && (
              <span>مهارت‌ها: {task.requiredSkills.join("، ")}</span>
            )}
          </p>
        </div>
        <Badge status={task.status} />
      </div>

      {getCachedUser()?.role === "manager" && (
        <DeadlineEditor task={task} busy={busy} onAct={onAct} />
      )}

      {(canStart || canComplete || canResume) && (
        <div className="mt-3 flex flex-wrap items-center gap-2 border-t border-slate-100 pt-3">
          {canStart && (
            <Button disabled={busy} onClick={() => onAct(task, { action: "start" })}>
              شروع کار
            </Button>
          )}
          {canComplete && !showComplete && (
            <Button variant="secondary" disabled={busy} onClick={() => setShowComplete(true)}>
              تکمیل کردن…
            </Button>
          )}
          {canResume && !showResume && (
            <Button disabled={busy} onClick={() => setShowResume(true)}>
              راهنمایی و ادامهٔ کار…
            </Button>
          )}

          {showComplete && (
            <form
              className="flex w-full gap-2"
              onSubmit={(e) => {
                e.preventDefault();
                if (notes.trim().length >= 15) {
                  onAct(task, { action: "complete", notes: notes.trim() });
                }
              }}
            >
              <Input
                autoFocus
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                placeholder='چه چیزی تحویل دادید؟ (هوش مصنوعی بررسی می‌کند — دقیق بنویسید)'
              />
              <Button type="submit" disabled={busy || notes.trim().length < 15}>
                ارسال برای بررسی
              </Button>
            </form>
          )}

          {showResume && (
            <form
              className="flex w-full gap-2"
              onSubmit={(e) => {
                e.preventDefault();
                if (guidance.trim()) {
                  onAct(task, { action: "resume", guidance: guidance.trim() });
                }
              }}
            >
              <Input
                autoFocus
                value={guidance}
                onChange={(e) => setGuidance(e.target.value)}
                placeholder="به تیم بگویید مشکل چگونه برطرف شود…"
              />
              <Button type="submit" disabled={busy || !guidance.trim()}>
                ادامهٔ کار
              </Button>
            </form>
          )}
        </div>
      )}

      {task.completedNotes && (
        <p className="mt-3 rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-600">
          یادداشت: {task.completedNotes}
          {task.verifiedAt && (
            <span className="mr-2 font-medium text-emerald-700">
              بررسی‌شده {new Date(task.verifiedAt).toLocaleTimeString("fa-IR")}
            </span>
          )}
        </p>
      )}
    </>
  );
}

function tehranDate(iso: string): string {
  return new Intl.DateTimeFormat("fa-IR", {
    timeZone: "Asia/Tehran",
    weekday: "short",
    day: "numeric",
    month: "long",
  }).format(new Date(iso));
}

function DeadlineEditor({
  task,
  busy,
  onAct,
}: {
  task: Task;
  busy: boolean;
  onAct: (task: Task, body: any) => void;
}) {
  const [open, setOpen] = useState(false);
  const [value, setValue] = useState(
    task.deadline ? new Date(task.deadline).toISOString().slice(0, 16) : ""
  );
  if (!open) {
    return (
      <div className="mt-2">
        <button
          onClick={() => setOpen(true)}
          className="text-[11px] text-slate-400 hover:text-indigo-600"
        >
          📅 تنظیم/تغییر مهلت
        </button>
      </div>
    );
  }
  return (
    <form
      className="mt-2 flex flex-wrap items-center gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        if (!value) return;
        onAct(task, { action: "__deadline__", notes: new Date(value).toISOString() });
        setOpen(false);
      }}
    >
      <input
        type="datetime-local"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        className="rounded-lg border border-slate-300 px-2.5 py-1.5 text-sm"
      />
      <Button
        variant="secondary"
        disabled={busy}
        onClick={() => {
          onAct(task, { action: "__deadline_clear__" });
          setOpen(false);
        }}
      >
        حذف مهلت
      </Button>
      <Button type="submit" disabled={busy || !value}>
        ذخیرهٔ مهلت
      </Button>
    </form>
  );
}
