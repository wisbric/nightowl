import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { BarChart, Bar, XAxis, YAxis, LabelList, ResponsiveContainer, Cell } from "recharts";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { SeverityBadge } from "@/components/ui/severity-badge";
import { StatusBadge } from "@/components/ui/status-badge";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { EmptyState } from "@/components/ui/empty-state";
import { OwlIcon } from "@/components/ui/owl-icon";
import { Link } from "@tanstack/react-router";
import { formatRelativeTime } from "@/lib/utils";
import type { AlertsResponse, AuditEntry, OnCallResponse, Roster, RostersResponse, UsersResponse } from "@/types/api";

function OnCallRow({ roster, usersById }: { roster: Roster; usersById: Record<string, string> }) {
  const { data, isLoading } = useQuery({
    queryKey: ["roster", roster.id, "oncall"],
    queryFn: () => api.get<OnCallResponse>(`/rosters/${roster.id}/oncall`),
  });

  function name(entry: { display_name?: string; user_id: string } | null | undefined): string {
    if (!entry) return "";
    return entry.display_name || usersById[entry.user_id] || entry.user_id.slice(0, 8);
  }

  return (
    <li className="flex items-center gap-2 rounded-md p-2 hover:bg-muted transition-colors">
      <span className="font-medium text-sm">{roster.name}</span>
      <span className="text-muted-foreground text-xs">({roster.timezone})</span>
      <span className="ml-auto flex items-center gap-2">
        {isLoading ? (
          <LoadingSpinner size="sm" label="" className="py-0" />
        ) : data?.primary ? (
          <>
            {data.source === "override" && (
              <Badge variant="outline" className="text-xs">Override</Badge>
            )}
            <span className="text-sm">{name(data.primary)}</span>
            {data.secondary && (
              <span className="text-xs text-muted-foreground">/ {name(data.secondary)}</span>
            )}
          </>
        ) : (
          <span className="text-xs text-muted-foreground">No one</span>
        )}
      </span>
    </li>
  );
}

function OnCallWidget({ rosters, usersById }: { rosters: Roster[]; usersById: Record<string, string> }) {
  if (rosters.length === 0) {
    return (
      <div className="mt-2 flex items-center gap-2 text-sm text-muted-foreground">
        <OwlIcon className="h-5 w-5 opacity-40" />
        <span>No rosters configured</span>
      </div>
    );
  }

  return (
    <ul className="mt-2 space-y-1">
      {rosters.map((roster) => (
        <OnCallRow key={roster.id} roster={roster} usersById={usersById} />
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

  const { data: activityData } = useQuery({
    queryKey: ["audit-log", "recent"],
    queryFn: () => api.get<AuditEntry[]>("/audit-log?limit=8"),
  });
  const activity = activityData ?? [];

  const { data: rostersData } = useQuery({
    queryKey: ["rosters"],
    queryFn: () => api.get<RostersResponse>("/rosters"),
  });
  const rosters = (rostersData?.rosters ?? []).filter((r) => r.is_active);

  const { data: usersData } = useQuery({
    queryKey: ["users"],
    queryFn: () => api.get<UsersResponse>("/users"),
  });

  const usersById = useMemo(() => {
    const map: Record<string, string> = {};
    for (const u of usersData?.users ?? []) {
      map[u.id] = u.display_name;
    }
    return map;
  }, [usersData]);

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
            <CardTitle className="text-sm font-medium text-muted-foreground">Recent Actions</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold">{activity.length}</div>
            <p className="mt-1 text-xs text-muted-foreground">latest audit entries</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium text-muted-foreground">On-Call Now</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold">{rosters.length}</div>
            <OnCallWidget rosters={rosters} usersById={usersById} />
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
                  <Bar dataKey="count" radius={[4, 4, 0, 0]} isAnimationActive={false}>
                    <LabelList dataKey="count" position="center" fill="#fff" fontWeight={600} fontSize={14} />
                    {severityData.map((entry) => (
                      <Cell key={entry.name} fill={entry.fill} cursor="default" />
                    ))}
                  </Bar>
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
              <EmptyState
                title="No active alerts"
                description="All clear right now."
                className="py-6"
              />
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

      <Card>
        <CardHeader>
          <CardTitle>Recent Activity</CardTitle>
        </CardHeader>
        <CardContent>
          {activity.length === 0 ? (
            <EmptyState
              title="No activity yet"
              description="Actions taken by your team will appear here."
              className="py-6"
            />
          ) : (
            <ul className="space-y-2">
              {activity.map((entry) => (
                <li key={entry.id} className="flex items-center gap-2 rounded-md p-2 hover:bg-muted transition-colors">
                  <Badge variant="outline" className="text-xs shrink-0">{entry.action}</Badge>
                  <span className="flex-1 truncate text-sm">
                    <span className="text-muted-foreground">{entry.resource}</span>
                    <span className="font-mono text-xs ml-1">{entry.resource_id.slice(0, 8)}</span>
                  </span>
                  <span className="text-xs text-muted-foreground whitespace-nowrap">{formatRelativeTime(entry.created_at)}</span>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
