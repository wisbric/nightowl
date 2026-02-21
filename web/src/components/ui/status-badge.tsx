import { cn, statusDot } from "@/lib/utils";

interface StatusBadgeProps {
  status: string;
  className?: string;
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  return (
    <span className={cn("inline-flex items-center gap-1.5 text-sm", className)}>
      <span className={cn("inline-block h-2 w-2 rounded-full", statusDot(status))} />
      <span className="capitalize">{status}</span>
    </span>
  );
}
