"use client";

import { FormEvent, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { setToken } from "@/lib/api";
import { landingFor } from "@/lib/auth";
import { Button, Card, Input } from "@/components/ui";

const API_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

export default function RegisterPage() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`${API_URL}/auth/register`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, email, password }),
      });
      const payload = await res.json();
      if (!res.ok) throw new Error(payload?.error?.message ?? "ثبت‌نام ناموفق بود");
      setToken(payload.data.token);
      window.localStorage.setItem("apa_user", JSON.stringify(payload.data.user));
      const role = payload.data.user?.role ?? "member";
      router.push(landingFor(role === "manager" ? { role: "manager" } as any : { role: "member" } as any));
    } catch (err: any) {
      setError(err.message ?? "ثبت‌نام ناموفق بود");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center p-6">
      <Card className="w-full max-w-sm p-6">
        <div className="mb-6 text-center">
          <img src="/logo.svg" alt="لوگو" className="mx-auto mb-3 h-16 w-16 object-contain" />
          <h1 className="text-xl font-semibold">ساخت حساب کاربری</h1>
          <p className="mt-1 text-sm text-slate-500">
            به عنوان عضو تیم به سازمان بپیوندید.
          </p>
        </div>

        <form onSubmit={submit} className="space-y-4">
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600">
              نام و نام خانوادگی
            </label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="مثلاً: علی حسنی"
              required
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600">
              ایمیل
            </label>
            <Input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@acme.test"
              required
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600">
              رمز عبور
            </label>
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="حداقل ۸ نویسه"
              minLength={8}
              required
            />
          </div>

          {error && (
            <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
          )}

          <Button type="submit" disabled={busy} className="w-full">
            {busy ? "در حال ساخت حساب…" : "ساخت حساب"}
          </Button>
        </form>

        <p className="mt-5 border-t border-slate-100 pt-4 text-center text-sm text-slate-500">
          از قبل حساب دارید؟{" "}
          <Link href="/login" className="font-medium text-indigo-600 hover:underline">
            ورود
          </Link>
        </p>
      </Card>
    </main>
  );
}
