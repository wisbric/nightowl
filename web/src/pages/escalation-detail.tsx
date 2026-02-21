import { useQuery, useMutation } from "@tanstack/react-query";
import { useParams, Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import type { EscalationPolicy } from "@/types/api";
import { useState } from "react";

interface DryRunStep {
  tier: number;
  delay_minutes: number;
  cumulative_minutes: number;
  notify_via: string[];
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
                <TableRow key={tier.tier}>
                  <TableCell><Badge variant="outline">L{tier.tier}</Badge></TableCell>
                  <TableCell className="text-sm">{tier.timeout_minutes} minutes</TableCell>
                  <TableCell className="text-sm capitalize">{tier.notify_via.join(", ")}</TableCell>
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
                  <span className="capitalize">{step.notify_via.join(", ")}</span>
                  <span className="text-muted-foreground">&rarr;</span>
                  <span>{step.targets.join(", ")}</span>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

    </div>
  );
}
