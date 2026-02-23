import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, Link } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select } from "@/components/ui/select";
import { SeverityBadge } from "@/components/ui/severity-badge";
import { Badge } from "@/components/ui/badge";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { Dialog, DialogHeader, DialogTitle, DialogContent } from "@/components/ui/dialog";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { formatRelativeTime } from "@/lib/utils";
import type { Incident, IncidentHistoryEntry, RunbooksResponse } from "@/types/api";
import { BookOpen } from "lucide-react";

interface IncidentForm {
  title: string;
  severity: string;
  category: string;
  symptoms: string;
  root_cause: string;
  solution: string;
  runbook_id: string;
  services: string;
  tags: string;
  error_patterns: string;
}

export function IncidentDetailPage() {
  const { incidentId } = useParams({ from: "/incidents/$incidentId" });
  const isNew = incidentId === "new";
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(isNew);
  const [mergeOpen, setMergeOpen] = useState(false);
  const [targetId, setTargetId] = useState("");

  const { data: incident, isLoading } = useQuery({
    queryKey: ["incident", incidentId],
    queryFn: () => api.get<Incident>(`/incidents/${incidentId}`),
    enabled: !isNew,
  });

  const { data: history } = useQuery({
    queryKey: ["incident-history", incidentId],
    queryFn: () => api.get<IncidentHistoryEntry[]>(`/incidents/${incidentId}/history`),
    enabled: !isNew,
  });

  const { data: runbooksData } = useQuery({
    queryKey: ["runbooks-all"],
    queryFn: () => api.get<RunbooksResponse>("/runbooks?limit=200"),
  });
  const runbooks = runbooksData?.items ?? [];

  useTitle(isNew ? "New Incident" : incident?.title ?? "Incident");

  const { register, handleSubmit, reset } = useForm<IncidentForm>({
    values: incident
      ? {
          title: incident.title,
          severity: incident.severity,
          category: incident.category,
          symptoms: incident.symptoms ?? "",
          root_cause: incident.root_cause ?? "",
          solution: incident.solution ?? "",
          runbook_id: incident.runbook_id ?? "",
          services: (incident.services ?? []).join(", "),
          tags: (incident.tags ?? []).join(", "),
          error_patterns: (incident.error_patterns ?? []).join("\n"),
        }
      : undefined,
  });

  const saveMutation = useMutation({
    mutationFn: (data: IncidentForm) => {
      const body = {
        ...data,
        runbook_id: data.runbook_id || null,
        services: data.services.split(",").map((s) => s.trim()).filter(Boolean),
        tags: data.tags.split(",").map((s) => s.trim()).filter(Boolean),
        error_patterns: data.error_patterns.split("\n").map((s) => s.trim()).filter(Boolean),
      };
      return isNew
        ? api.post<Incident>("/incidents", body)
        : api.put<Incident>(`/incidents/${incidentId}`, body);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["incidents"] });
      if (!isNew) {
        queryClient.invalidateQueries({ queryKey: ["incident", incidentId] });
        setEditing(false);
      }
    },
  });

  const mergeMutation = useMutation({
    mutationFn: (mergeTargetId: string) =>
      api.post(`/incidents/${incidentId}/merge`, { target_id: mergeTargetId }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["incidents"] });
      queryClient.invalidateQueries({ queryKey: ["incident", incidentId] });
      queryClient.invalidateQueries({ queryKey: ["incident-history", incidentId] });
      setMergeOpen(false);
      setTargetId("");
    },
  });

  function handleMergeSubmit() {
    const trimmed = targetId.trim();
    if (!trimmed) return;
    mergeMutation.mutate(trimmed);
  }

  if (!isNew && isLoading) return <LoadingSpinner size="lg" />;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/incidents" className="text-muted-foreground hover:text-foreground text-sm">&larr; Knowledge Base</Link>
      </div>

      {editing ? (
        <form onSubmit={handleSubmit((data) => saveMutation.mutate(data))} className="space-y-6">
          <Card>
            <CardHeader><CardTitle>{isNew ? "New Incident" : "Edit Incident"}</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium">Title</label>
                  <Input {...register("title")} required />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="text-sm font-medium">Severity</label>
                    <Select {...register("severity")} required>
                      <option value="critical">Critical</option>
                      <option value="major">Major</option>
                      <option value="warning">Warning</option>
                      <option value="info">Info</option>
                    </Select>
                  </div>
                  <div>
                    <label className="text-sm font-medium">Category</label>
                    <Input {...register("category")} />
                  </div>
                </div>
              </div>
              <div>
                <label className="text-sm font-medium">Symptoms</label>
                <Textarea {...register("symptoms")} rows={3} />
              </div>
              <div>
                <label className="text-sm font-medium">Root Cause</label>
                <Textarea {...register("root_cause")} rows={3} />
              </div>
              <div>
                <label className="text-sm font-medium">Solution</label>
                <Textarea {...register("solution")} rows={5} />
              </div>
              <div>
                <label className="text-sm font-medium">Linked Runbook (optional)</label>
                <Select {...register("runbook_id")}>
                  <option value="">No runbook</option>
                  {runbooks.map((rb) => (
                    <option key={rb.id} value={rb.id}>
                      {rb.title}{rb.category ? ` (${rb.category})` : ""}
                    </option>
                  ))}
                </Select>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium">Services (comma-separated)</label>
                  <Input {...register("services")} />
                </div>
                <div>
                  <label className="text-sm font-medium">Tags (comma-separated)</label>
                  <Input {...register("tags")} />
                </div>
              </div>
              <div>
                <label className="text-sm font-medium">Error Patterns (one per line)</label>
                <Textarea {...register("error_patterns")} rows={3} className="font-mono text-sm" />
              </div>
              <div className="flex gap-2">
                <Button type="submit" disabled={saveMutation.isPending}>
                  {saveMutation.isPending ? "Saving..." : "Save"}
                </Button>
                {!isNew && (
                  <Button type="button" variant="outline" onClick={() => { setEditing(false); reset(); }}>
                    Cancel
                  </Button>
                )}
              </div>
            </CardContent>
          </Card>
        </form>
      ) : incident ? (
        <>
          <div className="flex items-start justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold">{incident.title}</h1>
              <div className="mt-2 flex items-center gap-3">
                <SeverityBadge severity={incident.severity} />
                {incident.category && <Badge variant="outline">{incident.category}</Badge>}
                <span className="text-sm text-muted-foreground">Resolutions: {incident.resolution_count}</span>
              </div>
            </div>
            <div className="flex gap-2">
              <Button variant="outline" onClick={() => setMergeOpen(true)}>Merge</Button>
              <Button onClick={() => setEditing(true)}>Edit</Button>
            </div>
          </div>

          <Dialog open={mergeOpen} onClose={() => setMergeOpen(false)}>
            <DialogHeader>
              <DialogTitle>Merge Incident</DialogTitle>
            </DialogHeader>
            <DialogContent className="space-y-4">
              <p className="text-sm text-muted-foreground">
                Merge this incident into another. The target incident will absorb this incident's data.
              </p>
              <div>
                <label className="text-sm font-medium">Target Incident ID</label>
                <Input
                  value={targetId}
                  onChange={(e) => setTargetId(e.target.value)}
                  placeholder="Enter target incident ID"
                />
              </div>
              {mergeMutation.isError && (
                <p className="text-sm text-destructive">
                  {mergeMutation.error instanceof Error ? mergeMutation.error.message : "Merge failed"}
                </p>
              )}
              <div className="flex justify-end gap-2">
                <Button variant="outline" onClick={() => { setMergeOpen(false); setTargetId(""); }}>
                  Cancel
                </Button>
                <Button
                  onClick={handleMergeSubmit}
                  disabled={mergeMutation.isPending || !targetId.trim()}
                >
                  {mergeMutation.isPending ? "Merging..." : "Merge"}
                </Button>
              </div>
            </DialogContent>
          </Dialog>

          <div className="grid gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader><CardTitle>Symptoms</CardTitle></CardHeader>
              <CardContent><p className="text-sm whitespace-pre-wrap">{incident.symptoms || "—"}</p></CardContent>
            </Card>
            <Card>
              <CardHeader><CardTitle>Root Cause</CardTitle></CardHeader>
              <CardContent><p className="text-sm whitespace-pre-wrap">{incident.root_cause || "—"}</p></CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader><CardTitle>Solution</CardTitle></CardHeader>
            <CardContent><p className="text-sm whitespace-pre-wrap">{incident.solution || "—"}</p></CardContent>
          </Card>

          {incident.runbook_id && incident.runbook_title && (
            <Card>
              <CardHeader>
                <div className="flex items-center gap-2">
                  <BookOpen className="h-4 w-4 text-accent" />
                  <CardTitle>
                    <Link to="/runbooks/$runbookId" params={{ runbookId: incident.runbook_id }} className="hover:text-accent transition-colors">
                      {incident.runbook_title}
                    </Link>
                  </CardTitle>
                  <Badge variant="outline" className="text-xs">Runbook</Badge>
                </div>
              </CardHeader>
              {incident.runbook_content && (
                <CardContent>
                  <pre className="text-sm whitespace-pre-wrap font-mono bg-muted rounded-md p-4 overflow-x-auto">{incident.runbook_content}</pre>
                </CardContent>
              )}
            </Card>
          )}

          <div className="flex flex-wrap gap-4">
            {incident.services?.length > 0 && (
              <Card className="flex-1 min-w-[200px]">
                <CardHeader><CardTitle>Services</CardTitle></CardHeader>
                <CardContent><div className="flex flex-wrap gap-2">{incident.services.map((s) => <Badge key={s} variant="outline">{s}</Badge>)}</div></CardContent>
              </Card>
            )}
            {incident.tags?.length > 0 && (
              <Card className="flex-1 min-w-[200px]">
                <CardHeader><CardTitle>Tags</CardTitle></CardHeader>
                <CardContent><div className="flex flex-wrap gap-2">{incident.tags.map((t) => <Badge key={t} variant="secondary">{t}</Badge>)}</div></CardContent>
              </Card>
            )}
          </div>

          {incident.error_patterns?.length > 0 && (
            <Card>
              <CardHeader><CardTitle>Error Patterns</CardTitle></CardHeader>
              <CardContent>
                <ul className="space-y-1 font-mono text-sm">
                  {incident.error_patterns.map((p, i) => <li key={i} className="rounded bg-muted px-2 py-1">{p}</li>)}
                </ul>
              </CardContent>
            </Card>
          )}

          <div className="text-xs text-muted-foreground">
            Created {formatRelativeTime(incident.created_at)} &middot; Updated {formatRelativeTime(incident.updated_at)}
          </div>

          {/* History section */}
          <Card>
            <CardHeader><CardTitle>History</CardTitle></CardHeader>
            <CardContent>
              {history && history.length > 0 ? (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Field</TableHead>
                      <TableHead>Old Value</TableHead>
                      <TableHead>New Value</TableHead>
                      <TableHead>Changed By</TableHead>
                      <TableHead>When</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {history.map((entry) => (
                      <TableRow key={entry.id}>
                        <TableCell className="font-medium text-sm">{entry.field}</TableCell>
                        <TableCell className="text-sm text-muted-foreground max-w-[200px] truncate">
                          {entry.old_value || "—"}
                        </TableCell>
                        <TableCell className="text-sm max-w-[200px] truncate">
                          {entry.new_value || "—"}
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">{entry.changed_by}</TableCell>
                        <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                          {formatRelativeTime(entry.created_at)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              ) : (
                <p className="text-sm text-muted-foreground">No history recorded yet.</p>
              )}
            </CardContent>
          </Card>
        </>
      ) : (
        <p className="text-muted-foreground">Incident not found</p>
      )}
    </div>
  );
}
