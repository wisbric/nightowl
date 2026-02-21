import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { formatRelativeTime } from "@/lib/utils";
import type { EscalationPolicy } from "@/types/api";
import { ArrowUpCircle } from "lucide-react";

export function EscalationListPage() {
  useTitle("Escalation Policies");

  const { data: policies, isLoading } = useQuery({
    queryKey: ["escalation-policies"],
    queryFn: () => api.get<EscalationPolicy[]>("/escalation-policies"),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Escalation Policies</h1>

      <Card>
        <CardHeader><CardTitle>All Policies</CardTitle></CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-sm text-muted-foreground">Loading...</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead>Tiers</TableHead>
                  <TableHead>Updated</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(policies ?? []).map((policy) => (
                  <TableRow key={policy.id}>
                    <TableCell>
                      <Link to="/escalation/$policyId" params={{ policyId: policy.id }} className="font-medium text-sm hover:text-accent transition-colors flex items-center gap-2">
                        <ArrowUpCircle className="h-4 w-4" />
                        {policy.name}
                      </Link>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">{policy.description || "—"}</TableCell>
                    <TableCell>
                      <div className="flex gap-1">
                        {policy.tiers?.map((tier) => (
                          <Badge key={tier.level} variant="outline" className="text-xs">
                            L{tier.level}: {tier.timeout_minutes}m
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">{formatRelativeTime(policy.updated_at)}</TableCell>
                  </TableRow>
                ))}
                {(policies ?? []).length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-muted-foreground py-8">No escalation policies</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
