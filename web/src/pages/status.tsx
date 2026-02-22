import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { formatRelativeTime } from "@/lib/utils";
import type { StatusResponse } from "@/types/api";
import { Database, Server, Radio, Clock, Activity } from "lucide-react";

function StatusDot({ ok }: { ok: boolean }) {
  return (
    <span
      className={`inline-block h-3 w-3 rounded-full ${ok ? "bg-green-500" : "bg-red-500"}`}
    />
  );
}

export function StatusPage() {
  useTitle("System Status");

  const { data, isLoading } = useQuery({
    queryKey: ["status"],
    queryFn: () => api.get<StatusResponse>("/status"),
    refetchInterval: 10_000,
  });

  if (isLoading) {
    return <LoadingSpinner />;
  }

  if (!data) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold">System Status</h1>
        <Card>
          <CardContent className="py-8 text-center text-muted-foreground">
            Unable to reach the API. Is the backend running?
          </CardContent>
        </Card>
      </div>
    );
  }

  const overallOk = data.status === "ok";

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <h1 className="text-2xl font-bold">System Status</h1>
        <span
          className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-sm font-medium ${
            overallOk
              ? "bg-green-500/10 text-green-500"
              : "bg-red-500/10 text-red-500"
          }`}
        >
          <StatusDot ok={overallOk} />
          {overallOk ? "All systems operational" : "Degraded"}
        </span>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {/* Database */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Database</CardTitle>
            <Database className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <StatusDot ok={data.database === "ok"} />
              <span className="text-lg font-semibold capitalize">{data.database}</span>
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Latency: {data.database_latency_ms}ms
            </p>
          </CardContent>
        </Card>

        {/* Redis */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Redis</CardTitle>
            <Radio className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <StatusDot ok={data.redis === "ok"} />
              <span className="text-lg font-semibold capitalize">{data.redis}</span>
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Latency: {data.redis_latency_ms}ms
            </p>
          </CardContent>
        </Card>

        {/* Uptime */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Uptime</CardTitle>
            <Clock className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-lg font-semibold">{data.uptime}</div>
            <p className="text-xs text-muted-foreground mt-1">
              Since server start
            </p>
          </CardContent>
        </Card>

        {/* Version */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Version</CardTitle>
            <Server className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-lg font-semibold font-mono">{data.version}</div>
            <p className="text-xs text-muted-foreground mt-1">
              NightOwl API
            </p>
          </CardContent>
        </Card>

        {/* Last Alert */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Last Alert</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-lg font-semibold">
              {data.last_alert_at
                ? formatRelativeTime(data.last_alert_at)
                : "No alerts"}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              {data.last_alert_at
                ? new Date(data.last_alert_at).toLocaleString()
                : "No alerts have been received yet"}
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
