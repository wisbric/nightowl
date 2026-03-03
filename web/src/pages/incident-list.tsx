import { useState, useRef, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
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
import { Dialog, DialogHeader, DialogTitle, DialogContent } from "@/components/ui/dialog";
import { SeverityBadge } from "@/components/ui/severity-badge";
import { Badge } from "@/components/ui/badge";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRelativeTime } from "@/lib/utils";
import type { Incident, IncidentsResponse, SearchResponse, SearchResult } from "@/types/api";
import { Search, Download, BookOpen, Trash2, GitMerge, X } from "lucide-react";

interface SearchRow {
  incident: Incident;
  highlights?: {
    title_highlight: string;
    symptoms_highlight: string;
    solution_highlight: string;
  };
}

export function IncidentListPage() {
  useTitle("Incidents");
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [severityFilter, setSeverityFilter] = useState("");
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [showMergeDialog, setShowMergeDialog] = useState(false);
  const [mergeTargetId, setMergeTargetId] = useState("");
  const searchRef = useRef<HTMLInputElement>(null);

  useHotkey("/", useCallback(() => searchRef.current?.focus(), []));

  const isSearch = search.length >= 2;

  const toggleSelect = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  const toggleSelectAll = (ids: string[]) => {
    setSelectedIds((prev) => {
      if (ids.every((id) => prev.has(id))) return new Set();
      return new Set(ids);
    });
  };

  const bulkDeleteMutation = useMutation({
    mutationFn: () => Promise.all([...selectedIds].map((id) => api.delete(`/incidents/${id}`))),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["incidents"] });
      setSelectedIds(new Set());
      setShowDeleteConfirm(false);
    },
  });

  const bulkMergeMutation = useMutation({
    mutationFn: () =>
      Promise.all(
        [...selectedIds]
          .filter((id) => id !== mergeTargetId)
          .map((id) => api.post(`/incidents/${mergeTargetId}/merge`, { source_id: id })),
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["incidents"] });
      setSelectedIds(new Set());
      setShowMergeDialog(false);
      setMergeTargetId("");
    },
  });

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
        <h1 className="text-2xl font-bold">Incidents</h1>
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
          {/* Bulk action bar */}
          {selectedIds.size > 0 && (
            <div className="mb-4 flex items-center gap-3 rounded-md border border-accent/30 bg-accent/5 px-4 py-2">
              <span className="text-sm font-medium">{selectedIds.size} selected</span>
              <Button variant="outline" size="sm" onClick={() => setShowDeleteConfirm(true)}>
                <Trash2 className="h-3.5 w-3.5 mr-1" />
                Delete
              </Button>
              {selectedIds.size >= 2 && (
                <Button variant="outline" size="sm" onClick={() => setShowMergeDialog(true)}>
                  <GitMerge className="h-3.5 w-3.5 mr-1" />
                  Merge
                </Button>
              )}
              <Button variant="ghost" size="sm" onClick={() => setSelectedIds(new Set())}>
                <X className="h-3.5 w-3.5 mr-1" />
                Clear
              </Button>
            </div>
          )}

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
                title="No incidents yet"
                description="Create your first incident to start tracking operational issues."
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
                  <TableHead className="w-10">
                    <input
                      type="checkbox"
                      checked={rows.length > 0 && rows.every(({ incident }) => selectedIds.has(incident.id))}
                      onChange={() => toggleSelectAll(rows.map(({ incident }) => incident.id))}
                      className="rounded border-border"
                    />
                  </TableHead>
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
                    <TableCell>
                      <input
                        type="checkbox"
                        checked={selectedIds.has(inc.id)}
                        onChange={() => toggleSelect(inc.id)}
                        className="rounded border-border"
                      />
                    </TableCell>
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

      {/* Bulk delete confirmation */}
      <Dialog open={showDeleteConfirm} onClose={() => setShowDeleteConfirm(false)}>
        <DialogHeader>
          <DialogTitle>Delete Incidents</DialogTitle>
        </DialogHeader>
        <DialogContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Are you sure you want to delete <strong>{selectedIds.size}</strong> incident{selectedIds.size !== 1 ? "s" : ""}? This action cannot be undone.
          </p>
          {bulkDeleteMutation.isError && (
            <p className="text-sm text-destructive">
              {bulkDeleteMutation.error instanceof Error ? bulkDeleteMutation.error.message : "Delete failed"}
            </p>
          )}
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={() => setShowDeleteConfirm(false)}>Cancel</Button>
            <Button
              variant="destructive"
              onClick={() => bulkDeleteMutation.mutate()}
              disabled={bulkDeleteMutation.isPending}
            >
              {bulkDeleteMutation.isPending ? "Deleting..." : "Delete"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Bulk merge dialog */}
      <Dialog open={showMergeDialog} onClose={() => { setShowMergeDialog(false); setMergeTargetId(""); }}>
        <DialogHeader>
          <DialogTitle>Merge Incidents</DialogTitle>
        </DialogHeader>
        <DialogContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Select the target incident. All other selected incidents will be merged into it.
          </p>
          <Select
            value={mergeTargetId}
            onChange={(e) => setMergeTargetId(e.target.value)}
          >
            <option value="">Select target incident...</option>
            {rows
              .filter(({ incident }) => selectedIds.has(incident.id))
              .map(({ incident }) => (
                <option key={incident.id} value={incident.id}>
                  [{incident.severity}] {incident.title}
                </option>
              ))}
          </Select>
          {bulkMergeMutation.isError && (
            <p className="text-sm text-destructive">
              {bulkMergeMutation.error instanceof Error ? bulkMergeMutation.error.message : "Merge failed"}
            </p>
          )}
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={() => { setShowMergeDialog(false); setMergeTargetId(""); }}>Cancel</Button>
            <Button
              onClick={() => bulkMergeMutation.mutate()}
              disabled={bulkMergeMutation.isPending || !mergeTargetId}
            >
              {bulkMergeMutation.isPending ? "Merging..." : "Merge"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
