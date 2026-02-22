import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select } from "@/components/ui/select";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { SeverityBadge } from "@/components/ui/severity-badge";
import { StatusBadge } from "@/components/ui/status-badge";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRelativeTime } from "@/lib/utils";
import type { AlertsResponse } from "@/types/api";

export function AlertListPage() {
  useTitle("Alerts");
  const [statusFilter, setStatusFilter] = useState("");
  const [severityFilter, setSeverityFilter] = useState("");

  const params = new URLSearchParams();
  if (statusFilter) params.set("status", statusFilter);
  if (severityFilter) params.set("severity", severityFilter);
  params.set("limit", "100");

  const { data: alertsData, isLoading } = useQuery({
    queryKey: ["alerts", statusFilter, severityFilter],
    queryFn: () => api.get<AlertsResponse>(`/alerts?${params}`),
  });
  const alerts = alertsData?.alerts ?? [];

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Alerts</h1>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-4">
            <CardTitle className="flex-1">All Alerts</CardTitle>
            <Select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="w-40">
              <option value="">All statuses</option>
              <option value="firing">Firing</option>
              <option value="acknowledged">Acknowledged</option>
              <option value="resolved">Resolved</option>
            </Select>
            <Select value={severityFilter} onChange={(e) => setSeverityFilter(e.target.value)} className="w-40">
              <option value="">All severities</option>
              <option value="critical">Critical</option>
              <option value="major">Major</option>
              <option value="warning">Warning</option>
              <option value="info">Info</option>
            </Select>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <LoadingSpinner />
          ) : alerts.length === 0 ? (
            (statusFilter || severityFilter) ? (
              <EmptyState
                title="No matching alerts"
                description="Try adjusting the status or severity filters."
              />
            ) : (
              <EmptyState
                title="No alerts firing"
                description="All clear. Alerts from webhooks will appear here."
              />
            )
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Severity</TableHead>
                  <TableHead>Title</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Source</TableHead>
                  <TableHead>Service</TableHead>
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
                    <TableCell className="text-muted-foreground text-sm">{alert.labels?.service ?? "—"}</TableCell>
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
