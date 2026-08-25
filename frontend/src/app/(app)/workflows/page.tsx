"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import type { Workflow } from "@/lib/types";
import { Badge, Card, Spinner } from "@/components/ui";

export default function WorkflowsPage() {
  const [workflows, setWorkflows] = useState<Workflow[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .workflows()
      .then(setWorkflows)
      .catch((e) => setError(e.message));
  }, []);

  return (
    <div className="mx-auto max-w-4xl px-8 py-8">
      <h1 className="mb-1 text-xl font-semibold">گردش‌کارها</h1>
      <p className="mb-6 text-sm text-slate-500">
        درخواست هر مدیر، برنامهٔ پیشنهادی آن و چرخهٔ حیاتش.
      </p>

      {error && (
        <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
      )}
      {!workflows && !error && <Spinner />}

      <div className="space-y-3">
        {workflows?.map((wf) => (
          <Link key={wf.id} href={`/workflows/${wf.id}`} className="block">
            <Card className="flex items-center justify-between px-5 py-4 transition hover:border-indigo-300">
              <div>
                <p className="text-sm font-medium">{wf.title}</p>
                <p className="mt-0.5 line-clamp-1 max-w-xl text-xs text-slate-400">
                  {wf.intentText}
                </p>
                <p className="mt-1 text-xs text-slate-400">
                  {new Date(wf.createdAt).toLocaleString("fa-IR")}
                </p>
              </div>
              <Badge status={wf.status} />
            </Card>
          </Link>
        ))}
        {workflows && workflows.length === 0 && (
          <p className="text-sm text-slate-400">هنوز گردش‌کاری وجود ندارد.</p>
        )}
      </div>
    </div>
  );
}
