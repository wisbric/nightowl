import { cn, severityDot } from "@/lib/utils";

interface SeverityBadgeProps {
  severity: string;
  className?: string;
}

export function SeverityBadge({ severity, className }: SeverityBadgeProps) {
  return (
    <span className={cn("inline-flex items-center gap-1.5 text-sm", className)}>
      <span className={cn("inline-block h-2 w-2 rounded-full", severityDot(severity))} />
      <span className="capitalize">{severity}</span>
    </span>
  );
}
