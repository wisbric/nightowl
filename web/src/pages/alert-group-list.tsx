import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { SeverityBadge } from "@/components/ui/severity-badge";
import { Badge } from "@/components/ui/badge";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRelativeTime } from "@/lib/utils";
import type { AlertGroupsResponse } from "@/types/api";
import { Layers, Settings2 } from "lucide-react";

export function AlertGroupListPage() {
  useTitle("Alert Groups");
  const [statusFilter, setStatusFilter] = useState("active");

  const params = new URLSearchParams();
  if (statusFilter) params.set("status", statusFilter);

  const { data, isLoading } = useQuery({
    queryKey: ["alert-groups", statusFilter],
    queryFn: () => api.get<AlertGroupsResponse>(`/alert-groups?${params}`),
  });
  const groups = data?.groups ?? [];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Alert Groups</h1>
        <Link to="/alerts/groups/rules">
          <Button variant="outline" size="sm">
            <Settings2 className="h-4 w-4 mr-1" />
            Manage Rules
          </Button>
        </Link>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-4">
            <CardTitle className="flex-1">Groups</CardTitle>
            <Select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="w-40">
              <option value="">All statuses</option>
              <option value="active">Active</option>
              <option value="resolved">Resolved</option>
            </Select>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <LoadingSpinner />
          ) : groups.length === 0 ? (
            <EmptyState
              title="No alert groups"
              description="Groups appear automatically when alerts match grouping rules."
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Group</TableHead>
                  <TableHead>Severity</TableHead>
                  <TableHead>Alerts</TableHead>
                  <TableHead>Rule</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>First Alert</TableHead>
                  <TableHead>Last Alert</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {groups.map((group) => (
                  <TableRow key={group.id}>
                    <TableCell>
                      <Link
                        to="/alerts/groups/$groupId"
                        params={{ groupId: group.id }}
                        className="font-mono text-sm hover:text-accent transition-colors flex items-center gap-2"
                      >
                        <Layers className="h-4 w-4" />
                        {group.title}
                      </Link>
                    </TableCell>
                    <TableCell><SeverityBadge severity={group.max_severity} /></TableCell>
                    <TableCell>
                      <Badge variant="outline" className="text-xs">{group.alert_count}</Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">{group.rule_name}</TableCell>
                    <TableCell>
                      <Badge variant={group.status === "active" ? "default" : "outline"} className="text-xs">
                        {group.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {formatRelativeTime(group.first_alert_at)}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {formatRelativeTime(group.last_alert_at)}
                    </TableCell>
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
