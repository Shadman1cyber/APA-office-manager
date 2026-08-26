"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import type { Skill, User } from "@/lib/types";
import { Button, Card, Input, Spinner } from "@/components/ui";

interface TaskRow {
  title: string;
  description: string;
  topic: string;
  deadline: string; // datetime-local value
  assignedTo: string;
}

function toISO(localValue: string): string {
  if (!localValue) return "";
  return new Date(localValue).toISOString();
}

export default function NewWorkflowPage() {
  const router = useRouter();
  const [employees, setEmployees] = useState<User[] | null>(null);
  const [skills, setSkills] = useState<Skill[]>([]);
  const [rows, setRows] = useState<TaskRow[]>([
    { title: "", description: "", topic: "", deadline: "", assignedTo: "" },
  ]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    Promise.all([api.employees(), api.skills()])
      .then(([e, s]) => {
        setEmployees(e);
        setSkills(s);
      })
      .catch((err) => setError(err.message));
  }, []);

  useEffect(load, [load]);

  function updateRow(i: number, patch: Partial<TaskRow>) {
    setRows((rs) => rs.map((r, idx) => (idx === i ? { ...r, ...patch } : r)));
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const data = new FormData(e.currentTarget);
    setBusy(true);
    setError(null);
    try {
      const res = await api.createManualWorkflow({
        title: String(data.get("title") ?? ""),
        intent: String(data.get("intent") ?? ""),
        deadline: toISO(String(data.get("wfDeadline") ?? "")),
        tasks: rows
          .filter((r) => r.title.trim())
          .map((r) => ({
            title: r.title.trim(),
            description: r.description,
            topic: r.topic,
            requiredSkills: skills
              .filter((sk) => data.get(`sk_${sk.id}_${r.title}`) === "on" || data.get(`skillchk`) === "on")
              .map((sk) => sk.name),
            deadline: toISO(r.deadline),
            assignedTo: r.assignedTo || undefined,
          })),
      });
      router.push(`/workflows/${res.workflow.workflow.id}`);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  if (!employees || !skills) {
    return (
      <div className="p-10">
        <Spinner />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-4xl px-8 py-8">
      <h1 className="mb-1 text-xl font-semibold">گردش‌کار جدید (تعریف دستی)</h1>
      <p className="mb-6 text-sm text-slate-500">
        بدون واسطهٔ هوش مصنوعی؛ این گردش‌کار مستقیماً «تأییدشده» ساخته می‌شود و وظایف بی‌مسئول آن در
        بخش «قابل دریافت» اعضا ظاهر خواهد شد.
      </p>

      {error && (
        <p className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
      )}

      <form onSubmit={submit}>
        <Card className="mb-5 px-5 py-4">
          <div className="grid gap-3 sm:grid-cols-2">
            <Input name="title" placeholder="عنوان گردش‌کار *" required />
            <Input name="intent" placeholder="توضیح کوتاه (اختیاری)" />
            <label className="text-xs text-slate-500 sm:col-span-2">
              مهلت پیش‌فرض همهٔ وظایف (اختیاری):
              <input
                type="datetime-local"
                name="wfDeadline"
                className="ms-2 rounded-lg border border-slate-300 px-2.5 py-1.5 text-sm"
              />
            </label>
          </div>
        </Card>

        <div className="mb-5 space-y-3">
          {rows.map((row, i) => (
            <Card key={i} className="px-5 py-4">
              <div className="mb-2 flex items-center justify-between">
                <p className="text-xs font-semibold text-slate-500">وظیفهٔ {i + 1}</p>
                {rows.length > 1 && (
                  <button
                    type="button"
                    onClick={() => setRows(rows.filter((_, idx) => idx !== i))}
                    className="text-xs text-red-500 hover:underline"
                  >
                    حذف
                  </button>
                )}
              </div>
              <div className="grid gap-2 sm:grid-cols-2">
                <Input
                  placeholder={`عنوان وظیفهٔ ${i + 1} *`}
                  value={row.title}
                  onChange={(e) => updateRow(i, { title: e.target.value })}
                  required
                />
                <select
                  value={row.assignedTo}
                  onChange={(e) => updateRow(i, { assignedTo: e.target.value })}
                  className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-indigo-500"
                >
                  <option value="">مسئول: بدون تخصیص (قابل دریافت توسط اعضا)</option>
                  {employees.map((emp) => (
                    <option key={emp.id} value={emp.id}>
                      {emp.name} ({emp.role === "manager" ? "مدیر" : "عضو"})
                    </option>
                  ))}
                </select>
                <Input
                  placeholder="توضیح"
                  value={row.description}
                  onChange={(e) => updateRow(i, { description: e.target.value })}
                />
                <Input
                  placeholder="موضوع (برای یادگیری هوش مصنوعی)"
                  value={row.topic}
                  onChange={(e) => updateRow(i, { topic: e.target.value })}
                />
                <label className="text-xs text-slate-500 sm:col-span-2">
                  مهلت این وظیفه (خالی = مهلت پیش‌فرض):
                  <input
                    type="datetime-local"
                    value={row.deadline}
                    onChange={(e) => updateRow(i, { deadline: e.target.value })}
                    className="ms-2 rounded-lg border border-slate-300 px-2.5 py-1.5 text-sm"
                  />
                </label>
              </div>

              {skills.length > 0 && (
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {skills.map((sk) => (
                    <label
                      key={`${i}-${sk.id}`}
                      title={sk.description}
                      className="cursor-pointer rounded-full border border-slate-200 px-2.5 py-0.5 text-[11px] text-slate-600 has-checked:border-indigo-400 has-checked:bg-indigo-50 has-checked:text-indigo-700"
                    >
                      <input
                        type="checkbox"
                        name={`sk_${sk.id}_${row.title}`}
                        className="me-1 accent-indigo-600"
                      />
                      {sk.name}
                    </label>
                  ))}
                </div>
              )}
            </Card>
          ))}

          <Button
            variant="secondary"
            onClick={() =>
              setRows([...rows, { title: "", description: "", topic: "", deadline: "", assignedTo: "" }])
            }
          >
            + افزودن وظیفه
          </Button>
        </div>

        <Button type="submit" disabled={busy} className="w-full sm:w-auto">
          {busy ? "در حال ساخت…" : "ساخت گردش‌کار"}
        </Button>
      </form>
    </div>
  );
}
