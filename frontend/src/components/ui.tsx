"use client";

import type { ReactNode, ButtonHTMLAttributes, InputHTMLAttributes } from "react";

export function Card({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={`rounded-xl border border-slate-200 bg-white shadow-sm ${className}`}
    >
      {children}
    </div>
  );
}

type ButtonVariant = "primary" | "secondary" | "danger" | "ghost";

export function Button({
  children,
  variant = "primary",
  className = "",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant }) {
  const styles: Record<ButtonVariant, string> = {
    primary:
      "bg-indigo-600 text-white hover:bg-indigo-700 disabled:bg-indigo-300",
    secondary:
      "bg-white text-slate-800 border border-slate-300 hover:bg-slate-50",
    danger: "bg-red-600 text-white hover:bg-red-700 disabled:bg-red-300",
    ghost: "bg-transparent text-slate-600 hover:bg-slate-100",
  };
  return (
    <button
      {...props}
      className={`inline-flex items-center justify-center gap-1.5 rounded-lg px-3.5 py-2 text-sm font-medium transition disabled:cursor-not-allowed ${styles[variant]} ${className}`}
    >
      {children}
    </button>
  );
}

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      {...props}
      className={`w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 ${props.className ?? ""}`}
    />
  );
}

const statusLabels: Record<string, string> = {
  proposed: "پیشنهادی",
  approved: "تأییدشده",
  rejected: "ردشده",
  in_progress: "در حال انجام",
  completed: "تکمیل‌شده",
  failed: "ناموفق",
  pending: "در انتظار",
  assigned: "تخصیص‌یافته",
  verified: "بررسی‌شده",
  blocked: "مسدود",
  open: "باز",
  answered: "پاسخ داده شده",
  manager: "مدیر",
  member: "عضو",
};

const statusColors: Record<string, string> = {
  proposed: "bg-amber-100 text-amber-800",
  approved: "bg-blue-100 text-blue-800",
  rejected: "bg-slate-200 text-slate-600",
  in_progress: "bg-purple-100 text-purple-800",
  completed: "bg-emerald-100 text-emerald-800",
  failed: "bg-red-100 text-red-800",
  pending: "bg-slate-100 text-slate-700",
  assigned: "bg-blue-100 text-blue-800",
  verified: "bg-emerald-100 text-emerald-800",
  blocked: "bg-red-100 text-red-800",
  open: "bg-amber-100 text-amber-800",
  answered: "bg-emerald-100 text-emerald-800",
  manager: "bg-indigo-100 text-indigo-800",
  member: "bg-slate-100 text-slate-700",
};

export function Badge({ status }: { status: string }) {
  const cls = statusColors[status] ?? "bg-slate-100 text-slate-700";
  const label = statusLabels[status] ?? status.replace("_", " ");
  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${cls}`}
    >
      {label}
    </span>
  );
}

export function Confidence({ value }: { value: number }) {
  const pct = Math.round(value * 100);
  return (
    <span
      className={`text-xs font-medium ${
        pct >= 60 ? "text-emerald-700" : pct >= 30 ? "text-amber-700" : "text-slate-500"
      }`}
    >
      {pct}%
    </span>
  );
}

export function Spinner({ label }: { label?: string }) {
  return (
    <div className="flex items-center gap-2 text-sm text-slate-500">
      <span className="h-4 w-4 animate-spin rounded-full border-2 border-slate-300 border-t-indigo-600" />
      {label ?? "Loading…"}
    </div>
  );
}
