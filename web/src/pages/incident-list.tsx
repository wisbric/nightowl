import { useState, useRef, useCallback } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { useHotkey } from "@/hooks/use-hotkey";
import { downloadCSV } from "@/lib/csv";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { SeverityBadge } from "@/components/ui/severity-badge";
import { Badge } from "@/components/ui/badge";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRelativeTime } from "@/lib/utils";
import type { Incident, IncidentsResponse, SearchResponse, SearchResult } from "@/types/api";
import { Search, Download, BookOpen } from "lucide-react";

interface SearchRow {
  incident: Incident;
  highlights?: {
    title_highlight: string;
    symptoms_highlight: string;
    solution_highlight: string;
  };
}

export function IncidentListPage() {
  useTitle("Knowledge Base");
  const [search, setSearch] = useState("");
  const [severityFilter, setSeverityFilter] = useState("");
  const searchRef = useRef<HTMLInputElement>(null);

  useHotkey("/", useCallback(() => searchRef.current?.focus(), []));

  const isSearch = search.length >= 2;

  const { data, isLoading } = useQuery({
    queryKey: ["incidents", search, severityFilter],
    queryFn: async (): Promise<SearchRow[]> => {
      if (isSearch) {
        const res = await api.get<SearchResponse>(`/incidents/search?q=${encodeURIComponent(search)}&limit=50`);
        return res.results.map((r: SearchResult) => ({
          incident: {
            id: r.id,
            title: r.title,
            severity: r.severity,
            category: r.category,
            services: r.services ?? [],
            tags: r.tags ?? [],
            runbook_id: r.runbook_id,
            resolution_count: r.resolution_count,
            created_at: r.created_at,
            updated_at: r.created_at,
          } as Incident,
          highlights: {
            title_highlight: r.title_highlight,
            symptoms_highlight: r.symptoms_highlight,
            solution_highlight: r.solution_highlight,
          },
        }));
      }
      const params = new URLSearchParams();
      if (severityFilter) params.set("severity", severityFilter);
      params.set("limit", "50");
      const res = await api.get<IncidentsResponse>(`/incidents?${params}`);
      return res.items.map((inc) => ({ incident: inc }));
    },
  });
  const rows = data ?? [];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Knowledge Base</h1>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            onClick={() => {
              downloadCSV(
                "incidents.csv",
                ["Title", "Severity", "Category", "Services", "Occurrences", "Updated"],
                rows.map(({ incident: inc }) => [
                  inc.title,
                  inc.severity,
                  inc.category || "",
                  (inc.services ?? []).join("; "),
                  String(inc.resolution_count),
                  inc.updated_at,
                ]),
              );
            }}
            disabled={rows.length === 0}
          >
            <Download className="h-4 w-4 mr-1" />
            Export CSV
          </Button>
          <Link to="/incidents/$incidentId" params={{ incidentId: "new" }}>
            <Button>New Incident</Button>
          </Link>
        </div>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-4">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                ref={searchRef}
                placeholder="Search incidents... (press / to focus)"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-9"
              />
            </div>
            <Select value={severityFilter} onChange={(e) => setSeverityFilter(e.target.value)} className="w-40">
              <option value="">All severities</option>
              <option value="critical">Critical</option>
              <option value="major">Major</option>
              <option value="warning">Warning</option>
              <option value="info">Info</option>
            </Select>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <LoadingSpinner />
          ) : rows.length === 0 ? (
            isSearch ? (
              <EmptyState
                title="No matching incidents"
                description="No incidents match your search. Try different keywords."
              />
            ) : (
              <EmptyState
                title="Knowledge base is empty"
                description="Create your first incident to build operational knowledge."
                action={
                  <Link to="/incidents/$incidentId" params={{ incidentId: "new" }}>
                    <Button>New Incident</Button>
                  </Link>
                }
              />
            )
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Severity</TableHead>
                  <TableHead>Title</TableHead>
                  <TableHead>Category</TableHead>
                  <TableHead>Services</TableHead>
                  <TableHead>Occurrences</TableHead>
                  <TableHead>Updated</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map(({ incident: inc, highlights }) => (
                  <TableRow key={inc.id}>
                    <TableCell><SeverityBadge severity={inc.severity} /></TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1.5">
                        <Link to="/incidents/$incidentId" params={{ incidentId: inc.id }} className="text-sm hover:text-accent transition-colors">
                          {highlights?.title_highlight ? (
                            <span dangerouslySetInnerHTML={{ __html: highlights.title_highlight }} />
                          ) : (
                            inc.title
                          )}
                        </Link>
                        {inc.runbook_id && (
                          <span title="Has linked runbook"><BookOpen className="h-3.5 w-3.5 text-accent shrink-0" /></span>
                        )}
                      </div>
                      {highlights?.symptoms_highlight && (
                        <p
                          className="text-xs text-muted-foreground mt-1 line-clamp-2"
                          dangerouslySetInnerHTML={{ __html: highlights.symptoms_highlight }}
                        />
                      )}
                      {highlights?.solution_highlight && (
                        <p
                          className="text-xs text-muted-foreground mt-1 line-clamp-2"
                          dangerouslySetInnerHTML={{ __html: highlights.solution_highlight }}
                        />
                      )}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">{inc.category || "—"}</TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {(inc.services ?? []).slice(0, 3).map((s) => (
                          <Badge key={s} variant="outline" className="text-xs">{s}</Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell className="text-sm">{inc.resolution_count}</TableCell>
                    <TableCell className="text-muted-foreground text-sm whitespace-nowrap">{formatRelativeTime(inc.updated_at)}</TableCell>
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
