"use client";

import type { Task } from "@/lib/types";

export function DeadlineChip({ task }: { task: Task }) {
  if (!task.deadline) return null;
  const dl = new Date(task.deadline);
  const done = task.status === "verified" || task.status === "completed";
  const overdue = !done && dl.getTime() < Date.now();
  const soon = !overdue && !done && dl.getTime() - Date.now() < 2 * 86_400_000;
  const color = overdue
    ? "bg-red-100 text-red-700"
    : soon
      ? "bg-amber-100 text-amber-700"
      : "bg-slate-100 text-slate-600";
  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium ${color}`}>
      ⏰ مهلت:{" "}
      {new Intl.DateTimeFormat("fa-IR", {
        timeZone: "Asia/Tehran",
        weekday: "short",
        day: "numeric",
        month: "long",
      }).format(dl)}
      {overdue ? " (گذشته)" : soon ? " (نزدیک)" : ""}
    </span>
  );
}
