import type { User } from "./types";

export function getCachedUser(): User | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem("apa_user");
    if (!raw) return null;
    return JSON.parse(raw) as User;
  } catch {
    return null;
  }
}

export function isManager(): boolean {
  return getCachedUser()?.role === "manager";
}

export function landingFor(user: User): string {
  return user.role === "manager" ? "/chat" : "/tasks";
}
