import { useQuery } from "@tanstack/react-query";
import { useParams, Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { SeverityBadge } from "@/components/ui/severity-badge";
import { StatusBadge } from "@/components/ui/status-badge";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRelativeTime } from "@/lib/utils";
import type { AlertGroup, AlertsResponse } from "@/types/api";

export function AlertGroupDetailPage() {
  const { groupId } = useParams({ strict: false }) as { groupId: string };

  const { data: group, isLoading: groupLoading } = useQuery({
    queryKey: ["alert-group", groupId],
    queryFn: () => api.get<AlertGroup>(`/alert-groups/${groupId}`),
  });

  const { data: alertsData, isLoading: alertsLoading } = useQuery({
    queryKey: ["alert-group-alerts", groupId],
    queryFn: () => api.get<AlertsResponse>(`/alert-groups/${groupId}/alerts`),
  });
  const alerts = alertsData?.alerts ?? [];

  useTitle(group?.title ?? "Alert Group");

  if (groupLoading) return <LoadingSpinner size="lg" />;
  if (!group) return <p className="text-muted-foreground">Group not found</p>;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/alerts/groups" className="text-muted-foreground hover:text-foreground text-sm">&larr; Groups</Link>
      </div>

      <div>
        <h1 className="text-2xl font-bold font-mono">{group.title}</h1>
        <div className="mt-2 flex items-center gap-3 text-sm text-muted-foreground">
          <span>Rule: {group.rule_name}</span>
          <span>First alert {formatRelativeTime(group.first_alert_at)}</span>
          <span>Last alert {formatRelativeTime(group.last_alert_at)}</span>
        </div>
      </div>

      <div className="grid grid-cols-4 gap-4">
        <Card>
          <CardContent className="py-4">
            <div className="text-sm text-muted-foreground">Status</div>
            <div className="mt-1">
              <Badge variant={group.status === "active" ? "default" : "outline"}>{group.status}</Badge>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="py-4">
            <div className="text-sm text-muted-foreground">Max Severity</div>
            <div className="mt-1"><SeverityBadge severity={group.max_severity} /></div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="py-4">
            <div className="text-sm text-muted-foreground">Alert Count</div>
            <div className="mt-1 text-2xl font-bold">{group.alert_count}</div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="py-4">
            <div className="text-sm text-muted-foreground">Group Labels</div>
            <div className="mt-1 flex flex-wrap gap-1">
              {Object.entries(group.group_key_labels).map(([k, v]) => (
                <Badge key={k} variant="outline" className="text-xs font-mono">{k}={v}</Badge>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader><CardTitle>Alerts in Group</CardTitle></CardHeader>
        <CardContent>
          {alertsLoading ? (
            <LoadingSpinner />
          ) : alerts.length === 0 ? (
            <EmptyState title="No alerts" description="No alerts are assigned to this group yet." />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Severity</TableHead>
                  <TableHead>Title</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Source</TableHead>
                  <TableHead>Fired</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {alerts.map((alert) => (
                  <TableRow key={alert.id}>
                    <TableCell><SeverityBadge severity={alert.severity} /></TableCell>
                    <TableCell>
                      <Link to="/alerts/$alertId" params={{ alertId: alert.id }} className="font-mono text-sm hover:text-accent transition-colors">
                        {alert.title}
                      </Link>
                    </TableCell>
                    <TableCell><StatusBadge status={alert.status} /></TableCell>
                    <TableCell className="text-muted-foreground text-sm">{alert.source}</TableCell>
                    <TableCell className="text-muted-foreground text-sm whitespace-nowrap">{formatRelativeTime(alert.first_fired_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
