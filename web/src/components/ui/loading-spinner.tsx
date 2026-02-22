import { OwlIcon } from "@/components/ui/owl-icon";
import { cn } from "@/lib/utils";

const sizes = {
  sm: "h-6 w-6",
  md: "h-10 w-10",
  lg: "h-16 w-16",
} as const;

interface LoadingSpinnerProps {
  size?: keyof typeof sizes;
  label?: string;
  className?: string;
}

export function LoadingSpinner({
  size = "md",
  label = "Loading...",
  className,
}: LoadingSpinnerProps) {
  return (
    <div className={cn("flex flex-col items-center justify-center py-8", className)}>
      <OwlIcon className={cn("animate-spin", sizes[size])} />
      {label && (
        <p className="mt-2 text-sm text-muted-foreground">{label}</p>
      )}
    </div>
  );
}
