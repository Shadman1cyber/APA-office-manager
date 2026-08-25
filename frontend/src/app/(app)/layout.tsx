"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { api } from "@/lib/api";
import type { User } from "@/lib/types";

const memberNav = [
  { href: "/tasks", label: "وظایف من", icon: "✅" },
];

const managerBaseNav = [
  { href: "/chat", label: "دستیار", icon: "💬" },
  { href: "/workflows", label: "گردش‌کارها", icon: "🗂" },
  { href: "/tasks", label: "وظایف", icon: "✅" },
];

const managerNav = [
  { href: "/knowledge", label: "دانش سازمانی", icon: "🧠" },
  { href: "/team", label: "تیم", icon: "👥" },
];

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [user, setUser] = useState<User | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const token =
      typeof window !== "undefined" &&
      window.localStorage.getItem("apa_token");
    if (!token) {
      router.replace("/login");
      return;
    }
    api
      .me()
      .then((u) => {
        setUser(u);
        window.localStorage.setItem("apa_user", JSON.stringify(u));
        setReady(true);
      })
      .catch(() => {});
  }, [router]);

  function logout() {
    localStorage.removeItem("apa_token");
    localStorage.removeItem("apa_user");
    router.replace("/login");
  }

  if (!ready) {
    return (
      <div className="flex min-h-screen items-center justify-center text-slate-400">
        در حال بارگذاری…
      </div>
    );
  }

  return (
    <div className="flex min-h-screen">
      <aside className="flex w-60 flex-col border-r border-slate-200 bg-white">
        <div className="flex items-center gap-2.5 px-5 py-5">
          <img src="/logo.svg" alt="لوگو" className="h-10 w-10 object-contain" />
          <div>
            <p className="text-sm font-semibold">APA</p>
            <p className="text-xs text-slate-400">تخصیص هوشمند کار</p>
          </div>
        </div>

        <nav className="mt-2 flex-1 space-y-1 px-3">
          {[...(user?.role === "manager" ? managerBaseNav : memberNav), ...(user?.role === "manager" ? managerNav : [])].map((item) => {
            const active = pathname.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition ${
                  active
                    ? "bg-indigo-50 font-medium text-indigo-700"
                    : "text-slate-600 hover:bg-slate-50"
                }`}
              >
                <span>{item.icon}</span>
                {item.label}
              </Link>
            );
          })}
        </nav>

        {user && (
          <div className="border-t border-slate-100 p-4">
            <p className="truncate text-sm font-medium">{user.name}</p>
            <p className="text-xs text-slate-400">{user.role === "manager" ? "مدیر" : "عضو"}</p>
            <button
              onClick={logout}
              className="mt-3 text-xs text-slate-500 hover:text-red-600"
            >
              خروج
            </button>
          </div>
        )}
      </aside>

      <main className="flex-1 overflow-x-hidden">{children}</main>
    </div>
  );
}
