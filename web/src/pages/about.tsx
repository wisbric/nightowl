import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import type { StatusResponse } from "@/types/api";

export function AboutPage() {
  useTitle("About");

  const { data: status, isLoading } = useQuery({
    queryKey: ["status"],
    queryFn: () => api.get<StatusResponse>("/status"),
  });

  if (isLoading) return <LoadingSpinner size="lg" />;

  return (
    <div className="mx-auto max-w-lg space-y-6 py-8">
      <div className="flex flex-col items-center gap-4 text-center">
        <img src="/owl-logo.png" alt="NightOwl" className="h-20 w-auto" />
        <h1 className="text-3xl font-bold">NightOwl</h1>
        <p className="text-muted-foreground">
          Incident knowledge base, alert management, and on-call platform for
          24/7 operations teams.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Build Info</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <div className="flex justify-between">
            <span className="text-muted-foreground">Version</span>
            <span className="font-mono">{status?.version ?? "—"}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Commit</span>
            <span className="font-mono">{status?.commit_sha ?? "—"}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Uptime</span>
            <span className="font-mono">{status?.uptime ?? "—"}</span>
          </div>
        </CardContent>
      </Card>

      <p className="text-center text-xs text-muted-foreground">
        A Wisbric product
      </p>
    </div>
  );
}
