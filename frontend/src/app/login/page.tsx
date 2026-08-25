"use client";

import { FormEvent, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { api, setToken } from "@/lib/api";
import { landingFor } from "@/lib/auth";
import { Button, Card, Input } from "@/components/ui";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await api.login(email, password);
      setToken(res.token);
      window.localStorage.setItem("apa_user", JSON.stringify(res.user));
      router.push(landingFor(res.user));
    } catch (err: any) {
      setError(err.message ?? "ورود ناموفق بود");
    } finally {
      setBusy(false);
    }
  }


  return (
    <main className="flex min-h-screen items-center justify-center p-6">
      <Card className="w-full max-w-sm p-6">
        <div className="mb-6 text-center">
          <img src="/logo.svg" alt="لوگو" className="mx-auto mb-3 h-16 w-16 object-contain" />
          <h1 className="text-xl font-semibold">ورود به APA</h1>
          <p className="mt-1 text-sm text-slate-500">
            تخصیص هوشمند کار؛ سیستمی که از سازمان شما یاد می‌گیرد.
          </p>
        </div>

        <form onSubmit={submit} className="space-y-4">
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
              required
            />
          </div>

          {error && (
            <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
          )}

          <Button type="submit" disabled={busy} className="w-full">
            {busy ? "در حال ورود…" : "ورود"}
          </Button>
        </form>



        <p className="mt-5 text-center text-sm text-slate-500">
          حساب ندارید؟{" "}
          <Link href="/register" className="font-medium text-indigo-600 hover:underline">
            ساخت حساب کاربری
          </Link>
        </p>
      </Card>
    </main>
  );
}
