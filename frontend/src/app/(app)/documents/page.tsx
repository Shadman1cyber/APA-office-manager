"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { getCachedUser } from "@/lib/auth";
import type { DocumentItem } from "@/lib/types";
import { Button, Card, Input, Spinner } from "@/components/ui";

export default function DocumentsPage() {
  const manager = getCachedUser()?.role === "manager";
  const [docs, setDocs] = useState<DocumentItem[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [openId, setOpenId] = useState<string | null>(null);

  const load = useCallback(() => {
    api
      .documents()
      .then(setDocs)
      .catch((e) => setError(e.message));
  }, []);

  useEffect(() => {
    load();
    const timer = setInterval(load, 8000);
    return () => clearInterval(timer);
  }, [load]);

  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const contentEl = form.elements.namedItem("content") as HTMLTextAreaElement;
    const taskEl = form.elements.namedItem("taskId") as HTMLInputElement;
    setBusy(true);
    setError(null);
    try {
      await api.createDocument({
        content: contentEl.value,
        taskId: taskEl.value.trim() || undefined,
      });
      form.reset();
      load();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto max-w-4xl px-8 py-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">{manager ? "اسناد سازمان" : "اسناد من"}</h1>
          <p className="mt-1 text-sm text-slate-500">
            {manager
              ? "همهٔ اسنادی که هوش مصنوعی از گزارش کار اعضا تولید کرده است."
              : "چه‌کاری انجام داده‌اید را بنویسید؛ هوش مصنوعی آن را به سند رسمی تبدیل می‌کند."}
          </p>
        </div>
        <Button variant="secondary" onClick={load}>
          بازخوانی
        </Button>
      </div>

      {error && (
        <p className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
      )}
      {!docs && !error && <Spinner />}

      {!manager && (
        <Card className="mb-8 px-5 py-4">
          <h2 className="mb-3 text-sm font-semibold text-slate-700">ساخت سند جدید</h2>
          <form onSubmit={create} className="space-y-3">
            <Input name="taskId" placeholder="شناسه وظیفه (اختیاری)" />
            <textarea
              name="content"
              required
              minLength={10}
              rows={4}
              placeholder="مثلاً: امروز جلسه با تیم فروش داشتم، نیازهای آموزشی جمع‌آوری شد و گزارش اولیه نوشته شد…"
              className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100"
            />
            <Button type="submit" disabled={busy}>
              {busy ? "در حال تولید سند…" : "تولید خودکار سند"}
            </Button>
          </form>
        </Card>
      )}

      <div className="space-y-3">
        {docs?.map((d) => (
          <Card key={d.id} className="px-5 py-4">
            <button
              className="w-full text-right"
              onClick={() => setOpenId(openId === d.id ? null : d.id)}
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-sm font-medium">
                    {d.status === "generating" ? "⏳ در حال تولید… " : d.status === "failed" ? "⚠️ " : ""}
                    {d.title}
                  </p>
                  <p className="mt-1 text-xs text-slate-400">
                    {d.authorName ?? "—"}
                    {" · "}
                    {new Date(d.createdAt).toLocaleString("fa-IR")}
                  </p>
                </div>
                <span className="shrink-0 text-xs text-indigo-600">
                  {openId === d.id ? "بستن" : "مشاهده"}
                </span>
              </div>
            </button>

            {d.sourceNotes && openId === d.id && (
              <p className="mt-3 rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-500">
                یادداشت خام: {d.sourceNotes}
              </p>
            )}

            {openId === d.id && (
              <div className="mt-3 whitespace-pre-wrap border-t border-slate-100 pt-3 text-sm leading-7 text-slate-700">
                {d.body || "—"}
              </div>
            )}
          </Card>
        ))}
        {docs && docs.length === 0 && (
          <p className="text-sm text-slate-400">هنوز سندی تولید نشده است.</p>
        )}
      </div>
    </div>
  );
}
