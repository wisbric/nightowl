import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { formatRelativeTime } from "@/lib/utils";
import type { RunbooksResponse } from "@/types/api";

export function RunbookListPage() {
  useTitle("Runbooks");
  const [categoryFilter, setCategoryFilter] = useState("");

  const params = new URLSearchParams();
  if (categoryFilter) params.set("category", categoryFilter);
  params.set("limit", "100");

  const { data: runbooksData, isLoading } = useQuery({
    queryKey: ["runbooks", categoryFilter],
    queryFn: () => api.get<RunbooksResponse>(`/runbooks?${params}`),
  });
  const runbooks = runbooksData?.items ?? [];

  const categories = [...new Set(runbooks.map((r) => r.category).filter(Boolean))].sort();

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Runbooks</h1>
        <Link to="/runbooks/$runbookId" params={{ runbookId: "new" }}>
          <Button>Create Runbook</Button>
        </Link>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-4">
            <CardTitle className="flex-1">All Runbooks</CardTitle>
            <Select value={categoryFilter} onChange={(e) => setCategoryFilter(e.target.value)} className="w-48">
              <option value="">All categories</option>
              {categories.map((cat) => (
                <option key={cat} value={cat}>{cat}</option>
              ))}
            </Select>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-sm text-muted-foreground">Loading...</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Title</TableHead>
                  <TableHead>Category</TableHead>
                  <TableHead>Tags</TableHead>
                  <TableHead>Template</TableHead>
                  <TableHead>Updated</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {runbooks.map((runbook) => (
                  <TableRow key={runbook.id}>
                    <TableCell>
                      <Link to="/runbooks/$runbookId" params={{ runbookId: runbook.id }} className="text-sm hover:text-accent transition-colors">
                        {runbook.title}
                      </Link>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">{runbook.category || "—"}</TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {(runbook.tags ?? []).slice(0, 3).map((tag) => (
                          <Badge key={tag} variant="secondary" className="text-xs">{tag}</Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      {runbook.is_template && <Badge variant="outline">Template</Badge>}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm whitespace-nowrap">{formatRelativeTime(runbook.updated_at)}</TableCell>
                  </TableRow>
                ))}
                {runbooks.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center text-muted-foreground py-8">No runbooks found</TableCell>
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
