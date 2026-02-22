import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { EmptyState } from "@/components/ui/empty-state";
import type { RostersResponse } from "@/types/api";
import { Clock, Globe, Calendar, Plus } from "lucide-react";

export function RosterListPage() {
  useTitle("Rosters");

  const { data: rostersData, isLoading } = useQuery({
    queryKey: ["rosters"],
    queryFn: () => api.get<RostersResponse>("/rosters"),
  });
  const rosters = rostersData?.rosters ?? [];

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
                  <TableHead>Rotation</TableHead>
                  <TableHead>Handoff</TableHead>
                  <TableHead>Export</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rosters.map((roster) => (
                  <TableRow key={roster.id}>
                    <TableCell>
                      <Link to="/rosters/$rosterId" params={{ rosterId: roster.id }} className="font-medium text-sm hover:text-accent transition-colors">
                        {roster.name}
                      </Link>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      <span className="inline-flex items-center gap-1"><Globe className="h-3 w-3" />{roster.timezone}</span>
                    </TableCell>
                    <TableCell className="text-sm capitalize">{roster.rotation_type} ({roster.rotation_length}d)</TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      <span className="inline-flex items-center gap-1"><Clock className="h-3 w-3" />{roster.handoff_time}</span>
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
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
