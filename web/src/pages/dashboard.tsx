import { useQuery } from "@tanstack/react-query";
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from "recharts";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { SeverityBadge } from "@/components/ui/severity-badge";
import { StatusBadge } from "@/components/ui/status-badge";
import { Link } from "@tanstack/react-router";
import { formatRelativeTime } from "@/lib/utils";
import type { AlertsResponse, IncidentsResponse, OnCallResponse, Roster, RostersResponse } from "@/types/api";

function OnCallRow({ roster }: { roster: Roster }) {
  const { data, isLoading } = useQuery({
    queryKey: ["roster", roster.id, "oncall"],
    queryFn: () => api.get<OnCallResponse>(`/rosters/${roster.id}/oncall`),
  });

  return (
    <li className="flex items-center gap-2 rounded-md p-2 hover:bg-muted transition-colors">
      <span className="font-medium text-sm">{roster.name}</span>
      <span className="text-muted-foreground text-xs">({roster.timezone})</span>
      <span className="ml-auto flex items-center gap-2">
        {isLoading ? (
          <span className="text-xs text-muted-foreground">Loading...</span>
        ) : data?.on_call ? (
          <>
            {data.on_call.is_override && (
              <Badge variant="outline" className="text-xs">Override</Badge>
            )}
            <span className="text-sm">{data.on_call.roster_name ? data.on_call.user_id : "No one"}</span>
          </>
        ) : (
          <span className="text-xs text-muted-foreground">No one</span>
        )}
      </span>
    </li>
  );
}

function OnCallWidget({ rosters }: { rosters: Roster[] }) {
  if (rosters.length === 0) {
    return <p className="mt-1 text-sm text-muted-foreground">No rosters configured</p>;
  }

  return (
    <ul className="mt-2 space-y-1">
      {rosters.map((roster) => (
        <OnCallRow key={roster.id} roster={roster} />
      ))}
    </ul>
  );
}

export function DashboardPage() {
  useTitle("Dashboard");

  const { data: alertsData } = useQuery({
    queryKey: ["alerts", "active"],
    queryFn: () => api.get<AlertsResponse>("/alerts?status=firing&status=acknowledged&limit=100"),
  });
  const activeAlerts = alertsData?.alerts ?? [];

  const { data: incidentsData } = useQuery({
    queryKey: ["incidents", "recent"],
    queryFn: () => api.get<IncidentsResponse>("/incidents?limit=5"),
  });
  const incidents = incidentsData?.items ?? [];

  const { data: rostersData } = useQuery({
    queryKey: ["rosters"],
    queryFn: () => api.get<RostersResponse>("/rosters"),
  });
  const rosters = rostersData?.rosters ?? [];

  const criticalCount = activeAlerts.filter((a) => a.severity === "critical").length;
  const warningCount = activeAlerts.filter((a) => a.severity === "warning" || a.severity === "major").length;
  const infoCount = activeAlerts.filter((a) => a.severity === "info").length;

  const severityData = [
    { name: "Critical", count: criticalCount, fill: "#DC2626" },
    { name: "Warning", count: warningCount, fill: "#F59E0B" },
    { name: "Info", count: infoCount, fill: "#3B82F6" },
  ];

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Dashboard</h1>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium text-muted-foreground">Active Alerts</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold">{activeAlerts.length}</div>
            <div className="mt-1 flex gap-3 text-xs text-muted-foreground">
              <span className="text-severity-critical">{criticalCount} critical</span>
              <span className="text-severity-warning">{warningCount} warning</span>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium text-muted-foreground">Open Incidents</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold">{incidents.length}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium text-muted-foreground">On-Call Now</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold">{rosters.length}</div>
            <OnCallWidget rosters={rosters} />
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Alerts by Severity</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="h-64">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={severityData}>
                  <XAxis dataKey="name" tick={{ fill: "var(--color-muted-foreground)" }} />
                  <YAxis tick={{ fill: "var(--color-muted-foreground)" }} allowDecimals={false} />
                  <Tooltip
                    contentStyle={{ backgroundColor: "var(--color-card)", border: "1px solid var(--color-border)", color: "var(--color-card-foreground)" }}
                  />
                  <Bar dataKey="count" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Recent Alerts</CardTitle>
          </CardHeader>
          <CardContent>
            {activeAlerts.length === 0 ? (
              <p className="text-sm text-muted-foreground">No active alerts</p>
            ) : (
              <ul className="space-y-2">
                {activeAlerts.slice(0, 8).map((a) => (
                  <li key={a.id}>
                    <Link to="/alerts/$alertId" params={{ alertId: a.id }} className="flex items-center gap-2 rounded-md p-2 hover:bg-muted transition-colors">
                      <SeverityBadge severity={a.severity} />
                      <span className="flex-1 truncate font-mono text-sm">{a.title}</span>
                      <StatusBadge status={a.status} />
                      <span className="text-xs text-muted-foreground whitespace-nowrap">{formatRelativeTime(a.first_fired_at)}</span>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>

      {incidents.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Recent Incidents</CardTitle>
          </CardHeader>
          <CardContent>
            <ul className="space-y-2">
              {incidents.map((inc) => (
                <li key={inc.id}>
                  <Link to="/incidents/$incidentId" params={{ incidentId: inc.id }} className="flex items-center gap-2 rounded-md p-2 hover:bg-muted transition-colors">
                    <SeverityBadge severity={inc.severity} />
                    <span className="flex-1 truncate text-sm">{inc.title}</span>
                    <span className="text-xs text-muted-foreground whitespace-nowrap">{formatRelativeTime(inc.created_at)}</span>
                  </Link>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
