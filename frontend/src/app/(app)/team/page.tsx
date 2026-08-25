"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { User } from "@/lib/types";
import { Badge, Button, Card, Input, Spinner } from "@/components/ui";

export default function TeamPage() {
  const [employees, setEmployees] = useState<User[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    api
      .employees()
      .then(setEmployees)
      .catch((e) => setError(e.message));
  }, []);

  useEffect(load, [load]);

  async function createEmployee(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const data = new FormData(form);
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const created = await api.createEmployee({
        name: String(data.get("name") ?? ""),
        email: String(data.get("email") ?? ""),
        password: String(data.get("password") ?? ""),
        role: String(data.get("role") ?? "member"),
        skills: String(data.get("skills") ?? "")
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
      });
      setNotice(`${created.name} با نقش ${created.role === "manager" ? "مدیر" : "عضو"} به تیم اضافه شد.`);
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
      <h1 className="mb-1 text-xl font-semibold">تیم</h1>
      <p className="mb-6 text-sm text-slate-500">
        اعضای جدید بسازید و نقش و مهارت‌هایشان را مدیریت کنید. مهارت‌ها مستقیماً روی پیشنهادهای تخصیص هوش مصنوعی اثر می‌گذارند.
      </p>

      {error && (
        <p className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
      )}
      {notice && (
        <p className="mb-4 rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
          {notice}
        </p>
      )}

      <Card className="mb-8 px-5 py-4">
        <h2 className="mb-3 text-sm font-semibold text-slate-700">افزودن عضو جدید</h2>
        <form onSubmit={createEmployee} className="grid gap-3 sm:grid-cols-2">
          <Input name="name" placeholder="نام و نام خانوادگی" required />
          <Input name="email" type="email" placeholder="email@acme.test" required />
          <Input
            name="password"
            type="password"
            placeholder="رمز عبور موقت (حداقل ۸ نویسه)"
            minLength={8}
            required
          />
          <div className="flex gap-2">
            <select
              name="role"
              defaultValue="member"
              className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-indigo-500"
            >
              <option value="member">عضو</option>
              <option value="manager">مدیر</option>
            </select>
            <Input name="skills" placeholder="مهارت‌ها، با کاما جدا کنید" />
          </div>
          <div className="sm:col-span-2">
            <Button type="submit" disabled={busy}>
              {busy ? "در حال ایجاد…" : "ایجاد عضو"}
            </Button>
          </div>
        </form>
      </Card>

      {!employees && !error && <Spinner />}

      <div className="space-y-3">
        {employees?.map((emp) => (
          <EmployeeRow key={emp.id} employee={emp} onChanged={load} onError={setError} />
        ))}
      </div>
    </div>
  );
}

function EmployeeRow({
  employee,
  onChanged,
  onError,
}: {
  employee: User;
  onChanged: () => void;
  onError: (msg: string) => void;
}) {
  const [role, setRole] = useState(employee.role);
  const [skills, setSkills] = useState(employee.skills.join(", "));
  const [saving, setSaving] = useState(false);

  async function patch(body: { role?: string; skills?: string[] }) {
    setSaving(true);
    try {
      await api.updateEmployee(employee.id, body);
      onChanged();
    } catch (err: any) {
      onError(err.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card className="px-5 py-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-sm font-medium">{employee.name}</p>
          <p className="text-xs text-slate-400">{employee.email}</p>
        </div>
        <Badge status={role} />
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-2 border-t border-slate-100 pt-3">
        <select
          value={role}
          disabled={saving}
          onChange={(e) => {
            const next = e.target.value;
            setRole(next as User["role"]);
            patch({ role: next });
          }}
          className="rounded-lg border border-slate-300 bg-white px-2.5 py-1.5 text-sm outline-none focus:border-indigo-500"
        >
          <option value="member">عضو</option>
          <option value="manager">مدیر</option>
        </select>

        <Input
          value={skills}
          onChange={(e) => setSkills(e.target.value)}
          placeholder='مهارت‌ها، با کاما جدا کنید'
          className="!w-auto min-w-56 flex-1"
        />
        <Button
          variant="secondary"
          disabled={saving || skills === employee.skills.join(", ")}
          onClick={() => patch({ skills: skills.split(",").map((s) => s.trim()).filter(Boolean) })}
        >
          ذخیره مهارت‌ها
        </Button>
      </div>
    </Card>
  );
}
