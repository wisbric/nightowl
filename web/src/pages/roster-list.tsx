import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { EmptyState } from "@/components/ui/empty-state";
import type { Roster, RostersResponse, OnCallResponse, UsersResponse } from "@/types/api";
import { Globe, Calendar, Plus } from "lucide-react";

const DAYS_SHORT = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

function RosterListRow({ roster, usersById }: { roster: Roster; usersById: Record<string, string> }) {
  const { data, isLoading } = useQuery({
    queryKey: ["roster", roster.id, "oncall"],
    queryFn: () => api.get<OnCallResponse>(`/rosters/${roster.id}/oncall`),
  });

  function name(entry: { display_name?: string; user_id: string } | null | undefined): string {
    if (!entry) return "";
    return entry.display_name || usersById[entry.user_id] || entry.user_id.slice(0, 8);
  }

  return (
    <TableRow className={roster.is_active ? "" : "opacity-60"}>
      <TableCell>
        <Link to="/rosters/$rosterId" params={{ rosterId: roster.id }} className="font-medium text-sm hover:text-accent transition-colors">
          {roster.name}
        </Link>
      </TableCell>
      <TableCell className="text-sm text-muted-foreground">
        <span className="inline-flex items-center gap-1"><Globe className="h-3 w-3" />{roster.timezone}</span>
      </TableCell>
      <TableCell className="text-sm text-muted-foreground">
        {DAYS_SHORT[roster.handoff_day]} {roster.handoff_time}
      </TableCell>
      <TableCell className="text-sm">
        {isLoading ? (
          <LoadingSpinner size="sm" label="" className="py-0" />
        ) : data?.primary ? (
          <span>{name(data.primary)}</span>
        ) : (
          <span className="text-muted-foreground">None</span>
        )}
      </TableCell>
      <TableCell className="text-sm">
        {isLoading ? null : data?.secondary ? (
          <span className="text-muted-foreground">{name(data.secondary)}</span>
        ) : (
          <span className="text-muted-foreground">&mdash;</span>
        )}
      </TableCell>
      <TableCell>
        {roster.is_active ? (
          <Badge variant="default" className="bg-green-600/20 text-green-400 border-green-600/30">Active</Badge>
        ) : (
          <Badge variant="secondary">Ended {roster.end_date || "\u2014"}</Badge>
        )}
      </TableCell>
      <TableCell>
        <a
          href={`/api/v1/rosters/${roster.id}/export.ics`}
          className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
          download
        >
          <Calendar className="h-3 w-3" />
          iCal
        </a>
      </TableCell>
    </TableRow>
  );
}

export function RosterListPage() {
  useTitle("Rosters");

  const { data: rostersData, isLoading } = useQuery({
    queryKey: ["rosters"],
    queryFn: () => api.get<RostersResponse>("/rosters"),
  });
  const rosters = rostersData?.rosters ?? [];

  const { data: usersData } = useQuery({
    queryKey: ["users"],
    queryFn: () => api.get<UsersResponse>("/users"),
  });

  const usersById = useMemo(() => {
    const map: Record<string, string> = {};
    for (const u of usersData?.users ?? []) {
      map[u.id] = u.display_name;
    }
    return map;
  }, [usersData]);

  // Sort active rosters first.
  const sortedRosters = [...rosters].sort((a, b) => {
    const aActive = a.is_active;
    const bActive = b.is_active;
    if (aActive && !bActive) return -1;
    if (!aActive && bActive) return 1;
    return 0;
  });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Rosters</h1>
        <Link to="/rosters/$rosterId" params={{ rosterId: "new" }}>
          <Button>
            <Plus className="h-4 w-4" />
            Create Roster
          </Button>
        </Link>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>All Rosters</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <LoadingSpinner />
          ) : rosters.length === 0 ? (
            <EmptyState
              title="No rosters configured"
              description="Set up your first on-call roster to manage schedules."
              action={
                <Link to="/rosters/$rosterId" params={{ rosterId: "new" }}>
                  <Button>Create Roster</Button>
                </Link>
              }
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Timezone</TableHead>
                  <TableHead>Handoff</TableHead>
                  <TableHead>Primary</TableHead>
                  <TableHead>Secondary</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Export</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sortedRosters.map((roster) => (
                  <RosterListRow key={roster.id} roster={roster} usersById={usersById} />
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
