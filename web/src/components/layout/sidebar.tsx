import { Link, useMatchRoute } from "@tanstack/react-router";
import {
  LayoutDashboard,
  Bell,
  BookOpen,
  FileText,
  Users,
  ArrowUpCircle,
  Settings,
  Moon,
  Sun,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useTheme } from "@/hooks/use-theme";

const navItems = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/alerts", label: "Alerts", icon: Bell },
  { to: "/incidents", label: "Knowledge Base", icon: BookOpen },
  { to: "/runbooks", label: "Runbooks", icon: FileText },
  { to: "/rosters", label: "Rosters", icon: Users },
  { to: "/escalation", label: "Escalation", icon: ArrowUpCircle },
  { to: "/admin", label: "Admin", icon: Settings },
] as const;

export function Sidebar() {
  const matchRoute = useMatchRoute();
  const { theme, toggle } = useTheme();

  return (
    <aside className="flex h-screen w-56 flex-col bg-sidebar text-sidebar-foreground shrink-0">
      <div className="px-4 py-5 border-b border-white/10">
        <img src="/owl-logo.png" alt="NightOwl" className="h-8 w-auto" />
      </div>

      <nav className="flex-1 space-y-1 px-2 py-4">
        {navItems.map((item) => {
          const active = item.to === "/"
            ? matchRoute({ to: "/", fuzzy: false })
            : matchRoute({ to: item.to, fuzzy: true });
          return (
            <Link
              key={item.to}
              to={item.to}
              className={cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                active
                  ? "bg-sidebar-accent text-white"
                  : "text-sidebar-foreground/70 hover:bg-white/10 hover:text-sidebar-foreground"
              )}
            >
              <item.icon className="h-4 w-4" />
              {item.label}
            </Link>
          );
        })}
      </nav>

      <div className="border-t border-white/10 p-3">
        <button
          onClick={toggle}
          className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-sidebar-foreground/70 hover:bg-white/10 hover:text-sidebar-foreground transition-colors"
        >
          {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          {theme === "dark" ? "Light mode" : "Dark mode"}
        </button>
      </div>

      <div className="px-4 py-3 text-xs text-sidebar-foreground/40">
        NightOwl v0.1.0 — A Wisbric product
      </div>
    </aside>
  );
}
