import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { SeverityBadge } from "@/components/ui/severity-badge";
import { StatusBadge } from "@/components/ui/status-badge";
import { Badge } from "@/components/ui/badge";
import { formatRelativeTime } from "@/lib/utils";
import type { Alert } from "@/types/api";

export function AlertDetailPage() {
  const { alertId } = useParams({ from: "/alerts/$alertId" });
  const queryClient = useQueryClient();

  const { data: alert, isLoading } = useQuery({
    queryKey: ["alert", alertId],
    queryFn: () => api.get<Alert>(`/alerts/${alertId}`),
  });

  useTitle(alert?.title ?? "Alert");

  const ackMutation = useMutation({
    mutationFn: () => api.patch(`/alerts/${alertId}/acknowledge`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alert", alertId] });
      queryClient.invalidateQueries({ queryKey: ["alerts"] });
    },
  });

  const resolveMutation = useMutation({
    mutationFn: () => api.patch(`/alerts/${alertId}/resolve`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alert", alertId] });
      queryClient.invalidateQueries({ queryKey: ["alerts"] });
    },
  });

  if (isLoading) return <p className="text-muted-foreground">Loading...</p>;
  if (!alert) return <p className="text-muted-foreground">Alert not found</p>;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/alerts" className="text-muted-foreground hover:text-foreground text-sm">&larr; Alerts</Link>
      </div>

      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold font-mono">{alert.title}</h1>
          <div className="mt-2 flex items-center gap-3">
            <SeverityBadge severity={alert.severity} />
            <StatusBadge status={alert.status} />
            <span className="text-sm text-muted-foreground">Occurrences: {alert.occurrence_count}</span>
          </div>
        </div>
        <div className="flex gap-2">
          {alert.status === "firing" && (
            <Button onClick={() => ackMutation.mutate()} disabled={ackMutation.isPending}>
              {ackMutation.isPending ? "Acknowledging..." : "Acknowledge"}
            </Button>
          )}
          {alert.status !== "resolved" && (
            <Button variant="destructive" onClick={() => resolveMutation.mutate()} disabled={resolveMutation.isPending}>
              {resolveMutation.isPending ? "Resolving..." : "Resolve"}
            </Button>
          )}
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Details</CardTitle></CardHeader>
          <CardContent className="space-y-3 text-sm">
            {alert.description && <div><span className="text-muted-foreground">Description:</span><p className="mt-1">{alert.description}</p></div>}
            <div className="grid grid-cols-2 gap-2">
              <div><span className="text-muted-foreground">Source:</span> {alert.source}</div>
              <div><span className="text-muted-foreground">Service:</span> {alert.service || "—"}</div>
              <div><span className="text-muted-foreground">Cluster:</span> {alert.cluster || "—"}</div>
              <div><span className="text-muted-foreground">Namespace:</span> {alert.namespace || "—"}</div>
              <div><span className="text-muted-foreground">Created:</span> {formatRelativeTime(alert.created_at)}</div>
              <div><span className="text-muted-foreground">Last fired:</span> {formatRelativeTime(alert.last_fired_at)}</div>
            </div>
            {alert.acknowledged_by && (
              <div><span className="text-muted-foreground">Acknowledged by:</span> {alert.acknowledged_by} ({formatRelativeTime(alert.acknowledged_at!)})</div>
            )}
            {alert.fingerprint && <div><span className="text-muted-foreground">Fingerprint:</span> <code className="font-mono text-xs bg-muted px-1.5 py-0.5 rounded">{alert.fingerprint}</code></div>}
          </CardContent>
        </Card>

        <Card>
          <CardHeader><CardTitle>KB Match & Solution</CardTitle></CardHeader>
          <CardContent className="space-y-3 text-sm">
            {alert.matched_incident_id ? (
              <div>
                <span className="text-muted-foreground">Matched incident:</span>{" "}
                <Link to="/incidents/$incidentId" params={{ incidentId: alert.matched_incident_id }} className="text-accent hover:underline">{alert.matched_incident_id.slice(0, 8)}...</Link>
              </div>
            ) : (
              <p className="text-muted-foreground">No KB match found</p>
            )}
            {alert.suggested_solution && (
              <div>
                <span className="text-muted-foreground">Suggested solution:</span>
                <p className="mt-1 rounded-md bg-muted p-3">{alert.suggested_solution}</p>
              </div>
            )}
            {alert.agent_name && (
              <div><span className="text-muted-foreground">Agent:</span> {alert.agent_name} {alert.auto_resolved && <Badge variant="secondary">Auto-resolved</Badge>}</div>
            )}
          </CardContent>
        </Card>
      </div>

      {alert.labels && Object.keys(alert.labels).length > 0 && (
        <Card>
          <CardHeader><CardTitle>Labels</CardTitle></CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-2">
              {Object.entries(alert.labels).map(([k, v]) => (
                <Badge key={k} variant="outline"><span className="font-mono">{k}={v}</span></Badge>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
