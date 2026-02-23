import { useState, useRef, useEffect } from "react";
import { Link } from "@tanstack/react-router";
import { Key, Settings, LogOut, ChevronDown } from "lucide-react";
import { useAuth } from "@/contexts/auth-context";
import { Badge } from "@/components/ui/badge";

function getInitials(name: string): string {
  return name
    .split(/\s+/)
    .map((w) => w[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);
}

function getAvatarColor(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash);
  }
  const colors = [
    "bg-emerald-600",
    "bg-blue-600",
    "bg-violet-600",
    "bg-amber-600",
    "bg-rose-600",
    "bg-cyan-600",
    "bg-indigo-600",
    "bg-teal-600",
  ];
  return colors[Math.abs(hash) % colors.length];
}

export function UserMenu() {
  const { user, logout } = useAuth();
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const showLogout = !import.meta.env.DEV;

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    if (open) {
      document.addEventListener("mousedown", handleClickOutside);
      return () => document.removeEventListener("mousedown", handleClickOutside);
    }
  }, [open]);

  if (!user) return null;

  const initials = getInitials(user.display_name);
  const avatarColor = getAvatarColor(user.display_name);

  return (
    <div className="relative" ref={menuRef}>
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 rounded-lg px-2 py-1.5 text-sm transition-colors hover:bg-muted"
      >
        <div
          className={`flex h-8 w-8 items-center justify-center rounded-full text-xs font-bold text-white ${avatarColor}`}
        >
          {initials}
        </div>
        <span className="hidden font-medium sm:inline">{user.display_name}</span>
        <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
      </button>

      {open && (
        <div className="absolute right-0 top-full z-50 mt-1 w-64 rounded-lg border bg-card shadow-lg">
          <div className="border-b px-4 py-3">
            <div className="flex items-center gap-3">
              <div
                className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-sm font-bold text-white ${avatarColor}`}
              >
                {initials}
              </div>
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{user.display_name}</p>
                <p className="truncate text-xs text-muted-foreground">{user.email}</p>
              </div>
            </div>
            <div className="mt-2">
              <Badge variant="secondary" className="capitalize">
                {user.role}
              </Badge>
            </div>
          </div>

          <div className="p-1">
            <Link
              to="/settings/tokens"
              onClick={() => setOpen(false)}
              className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-foreground transition-colors hover:bg-muted"
            >
              <Key className="h-4 w-4 text-muted-foreground" />
              Personal Access Tokens
            </Link>
            <Link
              to="/settings"
              onClick={() => setOpen(false)}
              className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-foreground transition-colors hover:bg-muted"
            >
              <Settings className="h-4 w-4 text-muted-foreground" />
              Settings
            </Link>
          </div>

          {showLogout && (
            <div className="border-t p-1">
              <button
                onClick={() => {
                  setOpen(false);
                  logout();
                }}
                className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-foreground transition-colors hover:bg-muted"
              >
                <LogOut className="h-4 w-4 text-muted-foreground" />
                Sign out
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
