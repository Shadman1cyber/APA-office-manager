"use client";

import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { getCachedUser } from "@/lib/auth";
import type { ChatMessage, ChatReply } from "@/lib/types";
import { Button, Card, Input, Spinner } from "@/components/ui";

interface Message {
  role: "user" | "assistant";
  text: string;
  workflowId?: string;
  action?: ChatReply["action"];
}

const suggestions = [
  "تهیه گزارش آگاهی سایبری.",
  "تهیه خلاصه بازخورد مشتریان فصل گذشته.",
];

const dayFmt = new Intl.DateTimeFormat("en-CA", {
  timeZone: "Asia/Tehran",
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
});

function dayLabel(day: string): string {
  const todayKey = dayFmt.format(new Date());
  const yesterdayKey = dayFmt.format(new Date(Date.now() - 86_400_000));
  if (day === todayKey) return "امروز";
  if (day === yesterdayKey) return "دیروز";
  return new Intl.DateTimeFormat("fa-IR", {
    timeZone: "Asia/Tehran",
    weekday: "long",
    day: "numeric",
    month: "long",
  }).format(new Date(day + "T00:00:00"));
}

export default function ChatPage() {
  const router = useRouter();

  useEffect(() => {
    if (getCachedUser()?.role !== "manager") router.replace("/tasks");
  }, [router]);

  const [messages, setMessages] = useState<Message[]>([
    {
      role: "assistant",
      text:
        "سلام! من درخواست شما را به یک برنامهٔ پیشنهادی تبدیل می‌کنم، در صورت کمبود اطلاعات سؤال می‌پرسم و یاد می‌گیرم هر کار مسئول چه کسی است. امتحان کنید: «تهیه گزارش آگاهی سایبری.»"
    },
  ]);
  const [input, setInput] = useState("");
  const [days, setDays] = useState<{ day: string; count: number; preview: string }[]>([]);
  const [selectedDay, setSelectedDay] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const endRef = useRef<HTMLDivElement>(null);

  const loadDays = useCallback(() => {
    api.chatDays().then(setDays).catch(() => {});
  }, []);

  const backToLive = useCallback(() => {
    setSelectedDay(null);
    api
      .chatHistory()
      .then((history: ChatMessage[]) =>
        setMessages(
          history.length
            ? history.map((m) => ({
                role: m.role,
                text: m.text,
                workflowId: m.workflowId ?? undefined,
                action: (m.action as ChatReply["action"]) ?? undefined,
              }))
            : [
                {
                  role: "assistant" as const,
                  text: "گفتگوی جاری خالی است. یک درخواست بنویسید تا شروع کنیم.",
                },
              ]
        )
      )
      .catch(() => {});
  }, []);

  const viewDay = useCallback((day: string) => {
    setSelectedDay(day);
    setError(null);
    api
      .chatHistoryDay(day)
      .then((history) =>
        setMessages(
          history.map((m) => ({
            role: m.role,
            text: m.text,
            workflowId: m.workflowId ?? undefined,
          }))
        )
      )
      .catch(() => {});
  }, []);

  useEffect(() => {
    if (getCachedUser()?.role !== "manager") return;
    loadDays();
    api
      .chatHistory()
      .then((history: ChatMessage[]) => {
        if (!history.length) return;
        setMessages(
          history.map((m) => ({
            role: m.role,
            text: m.text,
            workflowId: m.workflowId ?? undefined,
            action: (m.action as ChatReply["action"]) ?? undefined,
          }))
        );
      })
      .catch(() => {});
  }, [loadDays]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  async function send(text: string) {
    const trimmed = text.trim();
    if (!trimmed || busy || selectedDay) return;
    setBusy(true);
    setError(null);
    setInput("");
    setMessages((m) => [...m, { role: "user", text: trimmed }]);
    try {
      const reply = await api.chat(trimmed);
      setMessages((m) => [
        ...m,
        { role: "assistant", text: reply.text, workflowId: reply.workflowId, action: reply.action },
      ]);
      loadDays();
    } catch (err: any) {
      setError(err.message ?? "خطایی رخ داد؛ لطفاً دوباره تلاش کنید.");
    } finally {
      setBusy(false);
    }
  }

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    send(input);
  }

  return (
    <div className="flex h-screen">
      <div className="mx-auto flex h-screen w-full max-w-3xl flex-col px-6 py-8">
        <div className="mb-1 flex items-center justify-between">
          <h1 className="text-xl font-semibold">دستیار هوشمند</h1>
          {selectedDay && (
            <Button variant="secondary" onClick={backToLive}>
              بازگشت به گفتگوی جاری
            </Button>
          )}
        </div>
        <p className="mb-6 text-sm text-slate-500">
          درخواست ← برنامه ← سؤال ← تأیید ← تخصیص ← یادگیری
        </p>

        {selectedDay && (
          <p className="mb-3 rounded-lg bg-slate-100 px-3 py-2 text-xs text-slate-600">
            در حال مشاهدهٔ تاریخچهٔ «{dayLabel(selectedDay)}» — برای ادامهٔ گفتگو بازگردید.
          </p>
        )}

        <div className="flex-1 space-y-4 overflow-y-auto pl-1">
          {messages.map((msg, i) => (
            <div
              key={i}
              className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
            >
              <Card
                className={`max-w-[85%] whitespace-pre-line px-4 py-3 text-sm ${
                  msg.role === "user"
                    ? "!bg-indigo-600 !text-white"
                    : msg.action === "created"
                      ? "border-indigo-200 bg-indigo-50/50"
                      : ""
                }`}
              >
                {msg.text}
                {msg.workflowId && msg.role === "assistant" && !selectedDay && (
                  <a
                    href={`/workflows/${msg.workflowId}`}
                    className="mt-2 block text-xs font-medium text-indigo-600 hover:underline"
                  >
                    مشاهدهٔ گردش‌کار
                  </a>
                )}
              </Card>
            </div>
          ))}
          {busy && <Spinner label="در حال فکر کردن…" />}
          {error && (
            <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
          )}
          {!selectedDay && messages.length === 0 && (
            <p className="py-8 text-center text-sm text-slate-400">گفتگویی ثبت نشده است.</p>
          )}
          <div ref={endRef} />
        </div>

        <div className="mt-4">
          {!selectedDay && messages.length <= 1 && (
            <div className="mb-3 flex flex-wrap gap-2">
              {suggestions.map((s) => (
                <button
                  key={s}
                  onClick={() => send(s)}
                  disabled={busy}
                  className="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs text-slate-600 hover:border-indigo-300 hover:text-indigo-700"
                >
                  {s}
                </button>
              ))}
            </div>
          )}
          <form onSubmit={onSubmit} className="flex gap-2">
            <Input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder={
                selectedDay
                  ? "در حالت تاریخچه، ارسال پیام غیرفعال است"
                  : 'درخواست خود را بنویسید، به سؤال پاسخ دهید یا «تأیید» بگویید…'
              }
              disabled={busy || !!selectedDay}
            />
            <Button type="submit" disabled={busy || !input.trim() || !!selectedDay}>
              ارسال
            </Button>
          </form>
        </div>
      </div>

      <aside className="hidden w-64 shrink-0 border-l border-slate-200 bg-white/70 p-4 lg:block">
        <h2 className="mb-3 text-sm font-semibold text-slate-700">تاریخچهٔ گفتگو</h2>
        {days.length === 0 && (
          <p className="text-xs text-slate-400">هنوز گفتگویی ثبت نشده است.</p>
        )}
        <div className="space-y-1.5 overflow-y-auto" style={{ maxHeight: "calc(100vh - 140px)" }}>
          {days.map((d) => (
            <button
              key={d.day}
              onClick={() => viewDay(d.day)}
              className={`w-full rounded-lg border px-3 py-2 text-right transition ${
                selectedDay === d.day
                  ? "border-indigo-300 bg-indigo-50"
                  : "border-transparent hover:border-slate-200 hover:bg-slate-50"
              }`}
            >
              <span className="block text-xs font-medium text-slate-700">
                {dayLabel(d.day)}
                <span className="ml-1 text-slate-400">({d.count} پیام)</span>
              </span>
              <span className="mt-0.5 block truncate text-[11px] text-slate-400">
                {d.preview}
              </span>
            </button>
          ))}
        </div>
      </aside>
    </div>
  );
}
