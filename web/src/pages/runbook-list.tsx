import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRelativeTime } from "@/lib/utils";
import type { BookOwlStatusResponse, BookOwlRunbookListResponse } from "@/types/api";
import { ExternalLink } from "lucide-react";

export function RunbookListPage() {
  useTitle("Runbooks");

  const { data: status, isLoading: statusLoading } = useQuery({
    queryKey: ["bookowl-status"],
    queryFn: () => api.get<BookOwlStatusResponse>("/bookowl/status"),
  });

  const integrated = status?.integrated ?? false;

  const { data: runbooksData, isLoading: runbooksLoading } = useQuery({
    queryKey: ["bookowl-runbooks"],
    queryFn: () => api.get<BookOwlRunbookListResponse>("/bookowl/runbooks?limit=100"),
    enabled: integrated,
  });
  const runbooks = runbooksData?.items ?? [];

  const isLoading = statusLoading || (integrated && runbooksLoading);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Runbooks</h1>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>All Runbooks</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <LoadingSpinner />
          ) : !integrated ? (
            <EmptyState
              title="BookOwl not connected"
              description="Connect BookOwl in Admin > Configuration to view and manage runbooks."
            />
          ) : runbooks.length === 0 ? (
            <EmptyState
              title="No runbooks yet"
              description="Create runbooks in BookOwl to see them here."
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Title</TableHead>
                  <TableHead>Tags</TableHead>
                  <TableHead>Updated</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {runbooks.map((runbook) => (
                  <TableRow key={runbook.id}>
                    <TableCell>
                      <a
                        href={runbook.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-1.5 text-sm hover:text-accent transition-colors"
                      >
                        {runbook.title}
                        <ExternalLink className="h-3 w-3 text-muted-foreground" />
                      </a>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {(runbook.tags ?? []).slice(0, 3).map((tag) => (
                          <Badge key={tag} variant="secondary" className="text-xs">{tag}</Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm whitespace-nowrap">{formatRelativeTime(runbook.updated_at)}</TableCell>
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
