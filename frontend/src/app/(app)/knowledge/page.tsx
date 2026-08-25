"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { Fact, PersonProfile } from "@/lib/types";
import { Card, Spinner } from "@/components/ui";

export default function KnowledgePage() {
  const [people, setPeople] = useState<PersonProfile[] | null>(null);
  const [facts, setFacts] = useState<Fact[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([api.people(), api.facts()])
      .then(([p, f]) => {
        setPeople(p);
        setFacts(f);
      })
      .catch((e) => setError(e.message));
  }, []);

  if (error) return <p className="p-10 text-sm text-red-600">{error}</p>;
  if (!people || !facts)
    return (
      <div className="p-10">
        <Spinner />
      </div>
    );

  return (
    <div className="mx-auto max-w-4xl px-8 py-8">
      <h1 className="mb-1 text-xl font-semibold">دانش سازمانی</h1>
      <p className="mb-6 text-sm text-slate-500">
        همهٔ آنچه سیستم دربارهٔ مسئولیت‌ها می‌داند؛ هر بار که مدیر به یک سؤال پاسخ می‌دهد یا وظیفه‌ای تحویل می‌شود، این دانش بزرگ‌تر می‌شود.
      </p>

      <section className="mb-8">
        <h2 className="mb-3 text-sm font-semibold text-slate-700">افراد</h2>
        <div className="grid gap-3 sm:grid-cols-2">
          {people.map((p) => (
            <Card key={p.id} className="px-5 py-4">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium">{p.name}</p>
                  <p className="text-xs text-slate-400">{p.role === "manager" ? "مدیر" : "عضو"}</p>
                </div>
              </div>
              <div className="mt-3 flex flex-wrap gap-1.5">
                {p.skills.map((s) => (
                  <span
                    key={s}
                    className="rounded-full bg-slate-100 px-2.5 py-0.5 text-xs text-slate-600"
                  >
                    {s}
                  </span>
                ))}
              </div>
              {p.ownedTopics.length > 0 && (
                <div className="mt-3 space-y-1 border-t border-slate-100 pt-3">
                  {p.ownedTopics.map((t) => (
                    <p key={t.subject} className="text-xs text-slate-500">
                      مسئول{" "}
                      <span className="font-medium text-indigo-700">{t.subject}</span>{" "}
                      ({Math.round(t.confidence * 100)}٪، {t.evidenceCount} بار مشاهده)
                    </p>
                  ))}
                </div>
              )}
            </Card>
          ))}
        </div>
      </section>

      <section>
        <h2 className="mb-3 text-sm font-semibold text-slate-700">دانش‌های ثبت‌شده</h2>
        <Card className="overflow-hidden">
          <table className="w-full text-right text-sm">
            <thead>
              <tr className="border-b border-slate-100 bg-slate-50/60 text-xs uppercase tracking-wide text-slate-400">
                <th className="px-4 py-2.5 font-medium">موضوع</th>
                <th className="px-4 py-2.5 font-medium">شخص</th>
                <th className="px-4 py-2.5 font-medium">اطمینان</th>
                <th className="px-4 py-2.5 font-medium">منبع</th>
                <th className="px-4 py-2.5 font-medium">دفعات مشاهده</th>
              </tr>
            </thead>
            <tbody>
              {facts.map((f) => (
                <tr key={f.id} className="border-b border-slate-50 last:border-0">
                  <td className="px-4 py-2.5 font-medium">{f.subject}</td>
                  <td className="px-4 py-2.5">{f.personName ?? f.personId.slice(0, 8)}</td>
                  <td className="px-4 py-2.5">{Math.round(f.confidence * 100)}٪</td>
                  <td className="px-4 py-2.5">
                    <span
                      className={`rounded-full px-2 py-0.5 text-xs ${
                        f.source === "learned"
                          ? "bg-emerald-100 text-emerald-700"
                          : "bg-slate-100 text-slate-600"
                      }`}
                    >
                      {f.source === "learned" ? "یادگرفته‌شده" : "اولیه"}
                    </span>
                  </td>
                  <td className="px-4 py-2.5">{f.evidenceCount} بار</td>
                </tr>
              ))}
            </tbody>
          </table>
          {facts.length === 0 && (
            <p className="px-4 py-6 text-center text-sm text-slate-400">
              هنوز دانشی ثبت نشده است.
            </p>
          )}
        </Card>
      </section>
    </div>
  );
}
