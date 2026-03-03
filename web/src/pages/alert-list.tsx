import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { downloadCSV } from "@/lib/csv";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { SeverityBadge } from "@/components/ui/severity-badge";
import { StatusBadge } from "@/components/ui/status-badge";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRelativeTime } from "@/lib/utils";
import type { AlertsResponse } from "@/types/api";
import { Download, CheckCircle, Eye, X } from "lucide-react";

export function AlertListPage() {
  useTitle("Alerts");
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState("");
  const [severityFilter, setSeverityFilter] = useState("");
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  const toggleSelect = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  const toggleSelectAll = (ids: string[]) => {
    setSelectedIds((prev) => {
      if (ids.every((id) => prev.has(id))) return new Set();
      return new Set(ids);
    });
  };

  const bulkAcknowledgeMutation = useMutation({
    mutationFn: () => Promise.all([...selectedIds].map((id) => api.patch(`/alerts/${id}/acknowledge`))),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alerts"] });
      setSelectedIds(new Set());
    },
  });

  const bulkResolveMutation = useMutation({
    mutationFn: () => Promise.all([...selectedIds].map((id) => api.patch(`/alerts/${id}/resolve`))),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alerts"] });
      setSelectedIds(new Set());
    },
  });

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
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                downloadCSV(
                  "alerts.csv",
                  ["Title", "Severity", "Status", "Source", "Service", "First Fired", "Last Fired"],
                  alerts.map((a) => [
                    a.title,
                    a.severity,
                    a.status,
                    a.source,
                    a.labels?.service ?? "",
                    a.first_fired_at,
                    a.last_fired_at,
                  ]),
                );
              }}
              disabled={alerts.length === 0}
            >
              <Download className="h-4 w-4 mr-1" />
              Export CSV
            </Button>
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
          {/* Bulk action bar */}
          {selectedIds.size > 0 && (
            <div className="mb-4 flex items-center gap-3 rounded-md border border-accent/30 bg-accent/5 px-4 py-2">
              <span className="text-sm font-medium">{selectedIds.size} selected</span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => bulkAcknowledgeMutation.mutate()}
                disabled={bulkAcknowledgeMutation.isPending}
              >
                <Eye className="h-3.5 w-3.5 mr-1" />
                {bulkAcknowledgeMutation.isPending ? "Acknowledging..." : "Acknowledge"}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => bulkResolveMutation.mutate()}
                disabled={bulkResolveMutation.isPending}
              >
                <CheckCircle className="h-3.5 w-3.5 mr-1" />
                {bulkResolveMutation.isPending ? "Resolving..." : "Resolve"}
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setSelectedIds(new Set())}>
                <X className="h-3.5 w-3.5 mr-1" />
                Clear
              </Button>
            </div>
          )}

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
                  <TableHead className="w-10">
                    <input
                      type="checkbox"
                      checked={alerts.length > 0 && alerts.every((a) => selectedIds.has(a.id))}
                      onChange={() => toggleSelectAll(alerts.map((a) => a.id))}
                      className="rounded border-border"
                    />
                  </TableHead>
                  <TableHead>Severity</TableHead>
                  <TableHead>Title</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Source</TableHead>
                  <TableHead>Group</TableHead>
                  <TableHead>Service</TableHead>
                  <TableHead>Fired</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {alerts.map((alert) => (
                  <TableRow key={alert.id}>
                    <TableCell>
                      <input
                        type="checkbox"
                        checked={selectedIds.has(alert.id)}
                        onChange={() => toggleSelect(alert.id)}
                        className="rounded border-border"
                      />
                    </TableCell>
                    <TableCell><SeverityBadge severity={alert.severity} /></TableCell>
                    <TableCell>
                      <Link to="/alerts/$alertId" params={{ alertId: alert.id }} className="font-mono text-sm hover:text-accent transition-colors">
                        {alert.title}
                      </Link>
                    </TableCell>
                    <TableCell><StatusBadge status={alert.status} /></TableCell>
                    <TableCell className="text-muted-foreground text-sm">{alert.source}</TableCell>
                    <TableCell>
                      {alert.alert_group_id ? (
                        <Link to="/alerts/groups/$groupId" params={{ groupId: alert.alert_group_id }} className="text-xs text-accent hover:underline">
                          Grouped
                        </Link>
                      ) : (
                        <span className="text-muted-foreground text-sm">—</span>
                      )}
                    </TableCell>
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
