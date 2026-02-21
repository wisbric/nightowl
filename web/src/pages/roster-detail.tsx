import { useQuery } from "@tanstack/react-query";
import { useParams, Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import type { Roster, Override } from "@/types/api";

export function RosterDetailPage() {
  const { rosterId } = useParams({ from: "/rosters/$rosterId" });

  const { data: roster, isLoading } = useQuery({
    queryKey: ["roster", rosterId],
    queryFn: () => api.get<Roster>(`/rosters/${rosterId}`),
  });

  const { data: overrides } = useQuery({
    queryKey: ["roster", rosterId, "overrides"],
    queryFn: () => api.get<Override[]>(`/rosters/${rosterId}/overrides`),
  });

  useTitle(roster?.name ?? "Roster");

  if (isLoading) return <p className="text-muted-foreground">Loading...</p>;
  if (!roster) return <p className="text-muted-foreground">Roster not found</p>;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/rosters" className="text-muted-foreground hover:text-foreground text-sm">&larr; Rosters</Link>
      </div>

      <h1 className="text-2xl font-bold">{roster.name}</h1>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Configuration</CardTitle></CardHeader>
          <CardContent className="space-y-2 text-sm">
            <div><span className="text-muted-foreground">Timezone:</span> {roster.timezone}</div>
            <div><span className="text-muted-foreground">Rotation:</span> <span className="capitalize">{roster.rotation_type}</span> ({roster.rotation_interval_days} days)</div>
            <div><span className="text-muted-foreground">Handoff:</span> {roster.handoff_time}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader><CardTitle>Members</CardTitle></CardHeader>
          <CardContent>
            {roster.members?.length > 0 ? (
              <ul className="space-y-2">
                {roster.members.map((m) => (
                  <li key={m.id} className="flex items-center gap-2 text-sm">
                    <span className="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-primary-foreground text-xs font-bold">
                      {m.position + 1}
                    </span>
                    <span>{m.display_name}</span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted-foreground">No members</p>
            )}
          </CardContent>
        </Card>
      </div>

      {overrides && overrides.length > 0 && (
        <Card>
          <CardHeader><CardTitle>Overrides</CardTitle></CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>User</TableHead>
                  <TableHead>Start</TableHead>
                  <TableHead>End</TableHead>
                  <TableHead>Reason</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {overrides.map((o) => (
                  <TableRow key={o.id}>
                    <TableCell className="text-sm font-medium">{o.display_name}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">{new Date(o.start_time).toLocaleString()}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">{new Date(o.end_time).toLocaleString()}</TableCell>
                    <TableCell className="text-sm">{o.reason || "—"}</TableCell>
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
