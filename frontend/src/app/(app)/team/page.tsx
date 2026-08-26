"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { Skill, User } from "@/lib/types";
import { Badge, Button, Card, Input, Spinner } from "@/components/ui";

export default function TeamPage() {
  const [employees, setEmployees] = useState<User[] | null>(null);
  const [skills, setSkills] = useState<Skill[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
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

  async function createEmployee(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const data = new FormData(form);
    const checkedSkills = Array.from(
      form.querySelectorAll<HTMLInputElement>('input[name="newEmpSkill"]:checked')
    ).map((el) => el.value);

    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const created = await api.createEmployee({
        name: String(data.get("name") ?? ""),
        email: String(data.get("email") ?? ""),
        password: String(data.get("password") ?? ""),
        role: String(data.get("role") ?? "member"),
        skills: checkedSkills,
      });
      setNotice(`${created.name} با نقش ${created.role === "manager" ? "مدیر" : "عضو"} اضافه شد.`);
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
      <h1 className="mb-1 text-xl font-semibold">تیم و مهارت‌ها</h1>
      <p className="mb-6 text-sm text-slate-500">
        عضو جدید بسازید، نقش‌ها را مدیریت کنید و فهرست مهارت‌هایی که هوش مصنوعی برای
        تخصیص کار استفاده می‌کند را تعریف کنید.
      </p>

      {error && (
        <p className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
      )}
      {notice && (
        <p className="mb-4 rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
          {notice}
        </p>
      )}

      {skills && (
        <SkillsCatalog skills={skills} onChanged={load} onError={setError} />
      )}

      <Card className="my-8 px-5 py-4">
        <h2 className="mb-3 text-sm font-semibold text-slate-700">افزودن عضو جدید</h2>
        <form onSubmit={createEmployee} className="grid gap-3 sm:grid-cols-2">
          <Input name="name" placeholder="نام و نام خانوادگی" required />
          <Input name="email" type="email" placeholder="email@company.ir" required />
          <Input
            name="password"
            type="password"
            placeholder="رمز عبور موقت (حداقل ۸ نویسه)"
            minLength={8}
            required
          />
          <select
            name="role"
            defaultValue="member"
            className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-indigo-500"
          >
            <option value="member">عضو</option>
            <option value="manager">مدیر</option>
          </select>

          <div className="sm:col-span-2">
            <p className="mb-2 text-xs font-medium text-slate-600">
              مهارت‌ها (از فهرست تعریف‌شده انتخاب کنید)
            </p>
            {!skills ? (
              <Spinner />
            ) : (
              <div className="flex flex-wrap gap-1.5">
                {skills.map((sk) => (
                  <label
                    key={sk.id}
                    title={sk.description}
                    className="cursor-pointer rounded-full border border-slate-200 px-3 py-1 text-xs text-slate-600 transition hover:border-indigo-300 has-checked:border-indigo-400 has-checked:bg-indigo-50 has-checked:text-indigo-700"
                  >
                    <input
                      type="checkbox"
                      name="newEmpSkill"
                      value={sk.name}
                      className="me-1 accent-indigo-600"
                    />
                    {sk.name}
                  </label>
                ))}
              </div>
            )}
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
          <EmployeeRow key={emp.id} employee={emp} skills={skills ?? []} onChanged={load} onError={setError} />
        ))}
      </div>
    </div>
  );
}

function SkillsCatalog({
  skills,
  onChanged,
  onError,
}: {
  skills: Skill[];
  onChanged: () => void;
  onError: (msg: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const data = new FormData(e.currentTarget);
    setBusy(true);
    try {
      await api.createSkill({
        name: String(data.get("name") ?? ""),
        description: String(data.get("description") ?? ""),
        keywords: String(data.get("keywords") ?? "")
          .split(",")
          .map((k) => k.trim())
          .filter(Boolean),
      });
      onChanged();
      setOpen(false);
    } catch (err: any) {
      onError(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card className="px-5 py-4">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-slate-700">
          مهارت‌های سازمان ({skills.length})
        </h2>
        <Button variant="secondary" onClick={() => setOpen(!open)}>
          {open ? "بستن" : "تعریف مهارت جدید"}
        </Button>
      </div>

      <div className="mt-3 flex flex-wrap gap-1.5">
        {skills.map((sk) => (
          <span
            key={sk.id}
            title={sk.description}
            className="rounded-full bg-indigo-50 px-3 py-1 text-xs font-medium text-indigo-700"
          >
            {sk.name}
          </span>
        ))}
      </div>
      <p className="mt-2 text-[11px] text-slate-400">
        توضیح هر مهارت، چیزی است که هوش مصنوعی برای تشخیص تناسب افراد با وظایف از آن استفاده می‌کند.
      </p>

      {open && (
        <form onSubmit={create} className="mt-4 space-y-3 border-t border-slate-100 pt-4">
          <Input name="name" placeholder="نام مهارت (مثلاً: تحلیل داده)" required />
          <textarea
            name="description"
            rows={3}
            required
            minLength={10}
            placeholder="توضیح کامل برای هوش مصنوعی: این مهارت یعنی چه، چه کارهایی را پوشش می‌دهد…"
            className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100"
          />
          <Input
            name="keywords"
            placeholder="کلیدواژه‌های تشخیص، جدا با کاما (مثلاً: داده، آمار، گزارش)"
            required
          />
          <Button type="submit" disabled={busy}>
            {busy ? "در حال ثبت…" : "ثبت مهارت"}
          </Button>
        </form>
      )}
    </Card>
  );
}

function EmployeeRow({
  employee,
  skills,
  onChanged,
  onError,
}: {
  employee: User;
  skills: Skill[];
  onChanged: () => void;
  onError: (msg: string) => void;
}) {
  const [role, setRole] = useState(employee.role);
  const [selected, setSelected] = useState<string[]>(employee.skills);
  const [saving, setSaving] = useState(false);
  const [expanded, setExpanded] = useState(false);

  const dirty =
    role !== employee.role ||
    JSON.stringify([...selected].sort()) !== JSON.stringify([...employee.skills].sort());

  function toggle(name: string) {
    setSelected((cur) =>
      cur.includes(name) ? cur.filter((s) => s !== name) : [...cur, name]
    );
  }

  async function save() {
    setSaving(true);
    try {
      await api.updateEmployee(employee.id, { skills: selected });
      onChanged();
    } catch (err: any) {
      onError(err.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card className="px-5 py-4">
      <button className="w-full text-right" onClick={() => setExpanded(!expanded)}>
        <div className="flex items-center justify-between">
          <div className="text-right">
            <p className="text-sm font-medium">{employee.name}</p>
            <p className="text-xs text-slate-400">{employee.email}</p>
          </div>
          <Badge status={role} />
        </div>
      </button>

      {expanded && (
        <div className="mt-3 space-y-3 border-t border-slate-100 pt-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs font-medium text-slate-600">نقش:</span>
            <select
              value={role}
              disabled={saving}
              onChange={(e) => {
                const next = e.target.value;
                setRole(next as User["role"]);
                api
                  .updateEmployee(employee.id, { role: next })
                  .then(onChanged)
                  .catch((err) => onError(err.message));
              }}
              className="rounded-lg border border-slate-300 bg-white px-2.5 py-1.5 text-sm outline-none focus:border-indigo-500"
            >
              <option value="member">عضو</option>
              <option value="manager">مدیر</option>
            </select>
          </div>

          <div>
            <p className="mb-1.5 text-xs font-medium text-slate-600">مهارت‌ها:</p>
            <div className="flex flex-wrap gap-1.5">
              {skills.map((sk) => {
                const active = selected.includes(sk.name);
                return (
                  <button
                    key={sk.id}
                    type="button"
                    onClick={() => toggle(sk.name)}
                    title={sk.description}
                    className={`rounded-full border px-3 py-1 text-xs transition ${
                      active
                        ? "border-indigo-400 bg-indigo-50 font-medium text-indigo-700"
                        : "border-slate-200 text-slate-500 hover:border-slate-300"
                    }`}
                  >
                    {sk.name}
                  </button>
                );
              })}
            </div>
          </div>

          {dirty && (
            <Button disabled={saving} onClick={save}>
              {saving ? "در حال ذخیره…" : "ذخیره تغییرات"}
            </Button>
          )}
        </div>
      )}
    </Card>
  );
}
