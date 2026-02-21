import { useQuery, useMutation } from "@tanstack/react-query";
import { useParams, Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import type { EscalationPolicy, EscalationEvent } from "@/types/api";
import { formatRelativeTime } from "@/lib/utils";
import { useState } from "react";

interface DryRunStep {
  tier: number;
  delay_minutes: number;
  cumulative_minutes: number;
  notify_type: string;
  targets: string[];
}

interface DryRunResponse {
  steps: DryRunStep[];
}

export function EscalationDetailPage() {
  const { policyId } = useParams({ from: "/escalation/$policyId" });
  const [dryRunResult, setDryRunResult] = useState<DryRunStep[] | null>(null);

  const { data: policy, isLoading } = useQuery({
    queryKey: ["escalation-policy", policyId],
    queryFn: () => api.get<EscalationPolicy>(`/escalation-policies/${policyId}`),
  });

  const { data: events } = useQuery({
    queryKey: ["escalation-policy", policyId, "events"],
    queryFn: () => api.get<EscalationEvent[]>(`/escalation-policies/${policyId}/events`),
  });

  useTitle(policy?.name ?? "Escalation Policy");

  const dryRunMutation = useMutation({
    mutationFn: () => api.post<DryRunResponse>(`/escalation-policies/${policyId}/dry-run`, {}),
    onSuccess: (data) => setDryRunResult(data.steps),
  });

  if (isLoading) return <p className="text-muted-foreground">Loading...</p>;
  if (!policy) return <p className="text-muted-foreground">Policy not found</p>;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/escalation" className="text-muted-foreground hover:text-foreground text-sm">&larr; Policies</Link>
      </div>

      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold">{policy.name}</h1>
          {policy.description && <p className="mt-1 text-sm text-muted-foreground">{policy.description}</p>}
        </div>
        <Button onClick={() => dryRunMutation.mutate()} disabled={dryRunMutation.isPending}>
          {dryRunMutation.isPending ? "Running..." : "Dry Run"}
        </Button>
      </div>

      <Card>
        <CardHeader><CardTitle>Escalation Tiers</CardTitle></CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Level</TableHead>
                <TableHead>Timeout</TableHead>
                <TableHead>Notification</TableHead>
                <TableHead>Targets</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {policy.tiers?.map((tier) => (
                <TableRow key={tier.level}>
                  <TableCell><Badge variant="outline">L{tier.level}</Badge></TableCell>
                  <TableCell className="text-sm">{tier.timeout_minutes} minutes</TableCell>
                  <TableCell className="text-sm capitalize">{tier.notify_type}</TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {tier.targets.map((t) => <Badge key={t} variant="secondary" className="text-xs">{t}</Badge>)}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {dryRunResult && (
        <Card>
          <CardHeader><CardTitle>Dry Run Result</CardTitle></CardHeader>
          <CardContent>
            <div className="space-y-3">
              {dryRunResult.map((step) => (
                <div key={step.tier} className="flex items-center gap-4 rounded-md border p-3 text-sm">
                  <Badge variant="outline">L{step.tier}</Badge>
                  <span className="text-muted-foreground">at {step.cumulative_minutes}m</span>
                  <span className="capitalize">{step.notify_type}</span>
                  <span className="text-muted-foreground">&rarr;</span>
                  <span>{step.targets.join(", ")}</span>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {events && events.length > 0 && (
        <Card>
          <CardHeader><CardTitle>Recent Escalation Events</CardTitle></CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Tier</TableHead>
                  <TableHead>Alert</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Target</TableHead>
                  <TableHead>When</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {events.map((event) => (
                  <TableRow key={event.id}>
                    <TableCell><Badge variant="outline">L{event.tier}</Badge></TableCell>
                    <TableCell className="font-mono text-xs">{event.alert_id.slice(0, 8)}...</TableCell>
                    <TableCell className="text-sm capitalize">{event.notify_type}</TableCell>
                    <TableCell className="text-sm">{event.target}</TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">{formatRelativeTime(event.notified_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
