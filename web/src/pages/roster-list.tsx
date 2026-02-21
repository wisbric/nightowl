import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import type { Roster, OnCallEntry } from "@/types/api";
import { Users, Clock, Globe } from "lucide-react";

export function RosterListPage() {
  useTitle("Rosters");

  const { data: rosters, isLoading } = useQuery({
    queryKey: ["rosters"],
    queryFn: () => api.get<Roster[]>("/rosters"),
  });

  const { data: oncall } = useQuery({
    queryKey: ["rosters", "oncall"],
    queryFn: () => api.get<OnCallEntry[]>("/rosters/on-call"),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Rosters</h1>

      {oncall && oncall.length > 0 && (
        <Card>
          <CardHeader><CardTitle>Currently On-Call</CardTitle></CardHeader>
          <CardContent>
            <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
              {oncall.map((entry) => (
                <div key={entry.roster_id} className="flex items-center gap-3 rounded-lg border p-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-full bg-accent text-accent-foreground font-bold text-sm">
                    {entry.display_name.charAt(0).toUpperCase()}
                  </div>
                  <div>
                    <p className="font-medium text-sm">{entry.display_name}</p>
                    <p className="text-xs text-muted-foreground">{entry.roster_name} &middot; {entry.timezone}</p>
                    {entry.is_override && <Badge variant="secondary" className="mt-1 text-xs">Override</Badge>}
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>All Rosters</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-sm text-muted-foreground">Loading...</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Timezone</TableHead>
                  <TableHead>Rotation</TableHead>
                  <TableHead>Handoff</TableHead>
                  <TableHead>Members</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(rosters ?? []).map((roster) => (
                  <TableRow key={roster.id}>
                    <TableCell>
                      <Link to="/rosters/$rosterId" params={{ rosterId: roster.id }} className="font-medium text-sm hover:text-accent transition-colors">
                        {roster.name}
                      </Link>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      <span className="inline-flex items-center gap-1"><Globe className="h-3 w-3" />{roster.timezone}</span>
                    </TableCell>
                    <TableCell className="text-sm capitalize">{roster.rotation_type} ({roster.rotation_interval_days}d)</TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      <span className="inline-flex items-center gap-1"><Clock className="h-3 w-3" />{roster.handoff_time}</span>
                    </TableCell>
                    <TableCell>
                      <span className="inline-flex items-center gap-1 text-sm"><Users className="h-3 w-3" />{roster.members?.length ?? 0}</span>
                    </TableCell>
                  </TableRow>
                ))}
                {(rosters ?? []).length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center text-muted-foreground py-8">No rosters configured</TableCell>
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
