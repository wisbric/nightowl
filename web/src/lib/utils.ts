import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatRelativeTime(date: string | Date): string {
  const now = new Date();
  const d = typeof date === "string" ? new Date(date) : date;
  const diff = now.getTime() - d.getTime();
  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (seconds < 60) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  if (hours < 24) return `${hours}h ago`;
  if (days < 7) return `${days}d ago`;
  return d.toLocaleDateString();
}

export function severityColor(severity: string): string {
  switch (severity.toLowerCase()) {
    case "critical":
      return "text-severity-critical";
    case "major":
    case "warning":
      return "text-severity-warning";
    case "info":
      return "text-severity-info";
    default:
      return "text-severity-ok";
  }
}

export function severityDot(severity: string): string {
  switch (severity.toLowerCase()) {
    case "critical":
      return "bg-severity-critical";
    case "major":
    case "warning":
      return "bg-severity-warning";
    case "info":
      return "bg-severity-info";
    default:
      return "bg-severity-ok";
  }
}

export function statusColor(status: string): string {
  switch (status.toLowerCase()) {
    case "firing":
      return "text-status-firing";
    case "acknowledged":
      return "text-status-ack";
    case "investigating":
      return "text-status-investigating";
    case "resolved":
      return "text-status-resolved";
    case "suppressed":
      return "text-status-suppressed";
    default:
      return "text-muted-foreground";
  }
}

export function statusDot(status: string): string {
  switch (status.toLowerCase()) {
    case "firing":
      return "bg-status-firing";
    case "acknowledged":
      return "bg-status-ack";
    case "investigating":
      return "bg-status-investigating";
    case "resolved":
      return "bg-status-resolved";
    case "suppressed":
      return "bg-status-suppressed";
    default:
      return "bg-muted-foreground";
  }
}
