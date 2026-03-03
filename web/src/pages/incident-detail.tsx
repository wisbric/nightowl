import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, Link, useNavigate } from "@tanstack/react-router";
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
import type {
  Incident, IncidentHistoryEntry, IncidentsResponse,
  BookOwlStatusResponse, BookOwlRunbookListResponse, BookOwlPostMortemResponse,
} from "@/types/api";
import { BookOpen, ExternalLink, FileText, GitMerge, Trash2 } from "lucide-react";

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

function HistoryDiff({ changeType, diff }: { changeType: string; diff: Record<string, unknown> }) {
  if (changeType === "created") {
    const title = diff.title as string | undefined;
    const severity = diff.severity as string | undefined;
    return <span className="text-muted-foreground">Incident created{title ? `: ${title}` : ""}{severity ? ` (${severity})` : ""}</span>;
  }

  if (changeType === "merged") {
    const sourceId = diff.merged_source_id as string | undefined;
    const targetId = diff.merged_into_id as string | undefined;
    const alerts = diff.reassigned_alerts as number | undefined;
    if (targetId) {
      return (
        <span className="text-muted-foreground">
          Merged into <Link to="/incidents/$incidentId" params={{ incidentId: targetId }} className="text-accent hover:underline">{targetId.slice(0, 8)}...</Link>
        </span>
      );
    }
    return (
      <span className="text-muted-foreground">
        Absorbed incident <Link to="/incidents/$incidentId" params={{ incidentId: sourceId! }} className="text-accent hover:underline">{sourceId!.slice(0, 8)}...</Link>
        {alerts ? ` (${alerts} alert${alerts !== 1 ? "s" : ""} reassigned)` : ""}
      </span>
    );
  }

  if (changeType === "archived") {
    return <span className="text-muted-foreground">Incident archived</span>;
  }

  // "updated" — show changed fields
  const fields = Object.entries(diff);
  if (fields.length === 0) return <span className="text-muted-foreground">No changes recorded</span>;
  return (
    <div className="space-y-1">
      {fields.map(([field, val]) => {
        const change = val as { old?: unknown; new?: unknown } | undefined;
        const oldStr = change?.old != null ? String(change.old) : "\u2014";
        const newStr = change?.new != null ? String(change.new) : "\u2014";
        return (
          <div key={field}>
            <span className="font-medium">{field.replace(/_/g, " ")}</span>{": "}
            <span className="text-muted-foreground line-through">{oldStr.length > 60 ? oldStr.slice(0, 60) + "..." : oldStr}</span>
            {" \u2192 "}
            <span>{newStr.length > 60 ? newStr.slice(0, 60) + "..." : newStr}</span>
          </div>
        );
      })}
    </div>
  );
}

export function IncidentDetailPage() {
  const { incidentId } = useParams({ strict: false }) as { incidentId: string };
  const isNew = incidentId === "new";
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [editing, setEditing] = useState(isNew);
  const [mergeOpen, setMergeOpen] = useState(false);
  const [targetId, setTargetId] = useState("");
  const [postMortemCreating, setPostMortemCreating] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

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

  const { data: mergedSources } = useQuery({
    queryKey: ["incident-merged-sources", incidentId],
    queryFn: () => api.get<Incident[]>(`/incidents/${incidentId}/merged-sources`),
    enabled: !isNew,
  });

  // BookOwl integration status
  const { data: bookowlStatus } = useQuery({
    queryKey: ["bookowl-status"],
    queryFn: () => api.get<BookOwlStatusResponse>("/bookowl/status"),
  });
  const bookowlIntegrated = bookowlStatus?.integrated ?? false;

  // Fetch runbooks: BookOwl if integrated, otherwise local
  const { data: bookowlRunbooks } = useQuery({
    queryKey: ["bookowl-runbooks"],
    queryFn: () => api.get<BookOwlRunbookListResponse>("/bookowl/runbooks?limit=200"),
    enabled: bookowlIntegrated,
  });

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

  // Fetch incidents for the merge dropdown (only when dialog is open).
  const { data: mergeIncidents } = useQuery({
    queryKey: ["incidents", "merge-candidates"],
    queryFn: () => api.get<IncidentsResponse>("/incidents?limit=100"),
    enabled: mergeOpen,
  });

  const mergeMutation = useMutation({
    mutationFn: (mergeTargetId: string) =>
      api.post(`/incidents/${mergeTargetId}/merge`, { source_id: incidentId }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["incidents"] });
      queryClient.invalidateQueries({ queryKey: ["incident", incidentId] });
      queryClient.invalidateQueries({ queryKey: ["incident-history", incidentId] });
      queryClient.invalidateQueries({ queryKey: ["incident-merged-sources"] });
      setMergeOpen(false);
      setTargetId("");
    },
  });

  const postMortemMutation = useMutation({
    mutationFn: async () => {
      setPostMortemCreating(true);
      const pm = await api.post<BookOwlPostMortemResponse>("/bookowl/post-mortems", {
        title: `Post-Mortem: ${incident!.title}`,
        space_slug: "post-mortems",
        incident: {
          id: incident!.id,
          title: incident!.title,
          severity: incident!.severity,
          root_cause: incident!.root_cause ?? "",
          solution: incident!.solution ?? "",
          created_at: incident!.created_at,
          resolved_at: incident!.updated_at,
          resolved_by: "",
        },
      });
      // Persist the post-mortem URL back to the incident
      await api.put(`/incidents/${incident!.id}/post-mortem-url`, { url: pm.url });
      return pm;
    },
    onSuccess: () => {
      setPostMortemCreating(false);
      queryClient.invalidateQueries({ queryKey: ["incident", incidentId] });
    },
    onError: () => {
      setPostMortemCreating(false);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => api.delete(`/incidents/${incidentId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["incidents"] });
      navigate({ to: "/incidents" });
    },
  });

  function handleMergeSubmit() {
    const trimmed = targetId.trim();
    if (!trimmed) return;
    mergeMutation.mutate(trimmed);
  }

  // Build runbook options for the dropdown
  const runbookOptions = bookowlIntegrated
    ? (bookowlRunbooks?.items ?? []).map((rb) => ({ id: rb.id, title: rb.title }))
    : [];

  // Can create post-mortem when BookOwl is integrated, incident has root_cause/solution, and no post-mortem exists yet
  const canCreatePostMortem = bookowlIntegrated && incident && !incident.post_mortem_url && (incident.root_cause || incident.solution);

  if (!isNew && isLoading) return <LoadingSpinner size="lg" />;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/incidents" className="text-muted-foreground hover:text-foreground text-sm">&larr; Incidents</Link>
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
                  {runbookOptions.map((rb) => (
                    <option key={rb.id} value={rb.id}>
                      {rb.title}
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
              {canCreatePostMortem && (
                <Button
                  variant="outline"
                  onClick={() => postMortemMutation.mutate()}
                  disabled={postMortemCreating}
                >
                  <FileText className="h-3.5 w-3.5 mr-1" />
                  {postMortemCreating ? "Creating..." : "Create Post-Mortem"}
                </Button>
              )}
              {incident.post_mortem_url && (
                <a href={incident.post_mortem_url} target="_blank" rel="noopener noreferrer">
                  <Button variant="outline">
                    <ExternalLink className="h-3.5 w-3.5 mr-1" />
                    View Post-Mortem
                  </Button>
                </a>
              )}
              <Button
                variant="outline"
                className="text-destructive hover:bg-destructive hover:text-destructive-foreground"
                onClick={() => setShowDeleteConfirm(true)}
              >
                <Trash2 className="h-3.5 w-3.5 mr-1" />
                Delete
              </Button>
              <Button variant="outline" onClick={() => setMergeOpen(true)}>Merge</Button>
              <Button onClick={() => setEditing(true)}>Edit</Button>
            </div>
          </div>

          {postMortemMutation.isError && (
            <div className="text-xs p-2 rounded bg-destructive/10 text-destructive">
              Failed to create post-mortem: {postMortemMutation.error instanceof Error ? postMortemMutation.error.message : "unknown error"}
            </div>
          )}

          <Dialog open={mergeOpen} onClose={() => setMergeOpen(false)}>
            <DialogHeader>
              <DialogTitle>Merge Incident</DialogTitle>
            </DialogHeader>
            <DialogContent className="space-y-4">
              <p className="text-sm text-muted-foreground">
                Merge this incident into another. The selected target incident will absorb this incident's data.
              </p>
              <div>
                <label className="text-sm font-medium">Target Incident</label>
                <Select
                  value={targetId}
                  onChange={(e) => setTargetId(e.target.value)}
                >
                  <option value="">Select an incident...</option>
                  {(mergeIncidents?.items ?? [])
                    .filter((inc) => inc.id !== incidentId)
                    .map((inc) => (
                      <option key={inc.id} value={inc.id}>
                        [{inc.severity}] {inc.title}
                      </option>
                    ))}
                </Select>
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

          <Dialog open={showDeleteConfirm} onClose={() => setShowDeleteConfirm(false)}>
            <DialogHeader>
              <DialogTitle>Delete Incident</DialogTitle>
            </DialogHeader>
            <DialogContent className="space-y-4">
              <p className="text-sm text-muted-foreground">
                Are you sure you want to delete <strong>{incident.title}</strong>? This action cannot be undone.
              </p>
              {deleteMutation.isError && (
                <p className="text-sm text-destructive">
                  {deleteMutation.error instanceof Error ? deleteMutation.error.message : "Delete failed"}
                </p>
              )}
              <div className="flex justify-end gap-2">
                <Button variant="outline" onClick={() => setShowDeleteConfirm(false)}>Cancel</Button>
                <Button
                  variant="destructive"
                  onClick={() => deleteMutation.mutate()}
                  disabled={deleteMutation.isPending}
                >
                  {deleteMutation.isPending ? "Deleting..." : "Delete"}
                </Button>
              </div>
            </DialogContent>
          </Dialog>

          {incident.merged_into_id && (
            <div className="rounded-md border border-accent/30 bg-accent/5 p-3 flex items-center gap-2">
              <GitMerge className="h-4 w-4 text-accent flex-shrink-0" />
              <span className="text-sm">
                This incident was merged into{" "}
                <Link
                  to="/incidents/$incidentId"
                  params={{ incidentId: incident.merged_into_id }}
                  className="text-accent hover:underline font-medium"
                >
                  {incident.merged_into_id.slice(0, 8)}...
                </Link>
              </span>
            </div>
          )}

          <div className="grid gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader><CardTitle>Symptoms</CardTitle></CardHeader>
              <CardContent><p className="text-sm whitespace-pre-wrap">{incident.symptoms || "\u2014"}</p></CardContent>
            </Card>
            <Card>
              <CardHeader><CardTitle>Root Cause</CardTitle></CardHeader>
              <CardContent><p className="text-sm whitespace-pre-wrap">{incident.root_cause || "\u2014"}</p></CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader><CardTitle>Solution</CardTitle></CardHeader>
            <CardContent><p className="text-sm whitespace-pre-wrap">{incident.solution || "\u2014"}</p></CardContent>
          </Card>

          {incident.runbook_id && incident.runbook_title && (
            <Card>
              <CardHeader>
                <div className="flex items-center gap-2">
                  <BookOpen className="h-4 w-4 text-accent" />
                  <CardTitle>
                    {bookowlIntegrated ? (
                      <Link to="/runbooks/$runbookId" params={{ runbookId: incident.runbook_id }} className="hover:text-accent transition-colors">
                        {incident.runbook_title}
                      </Link>
                    ) : (
                      <span>{incident.runbook_title}</span>
                    )}
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

          {/* Merged Incidents section */}
          {mergedSources && mergedSources.length > 0 && (
            <Card>
              <CardHeader>
                <div className="flex items-center gap-2">
                  <GitMerge className="h-4 w-4 text-accent" />
                  <CardTitle>Merged Incidents</CardTitle>
                </div>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Incident</TableHead>
                      <TableHead>Severity</TableHead>
                      <TableHead>Category</TableHead>
                      <TableHead>Merged</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {mergedSources.map((src) => (
                      <TableRow key={src.id}>
                        <TableCell>
                          <Link
                            to="/incidents/$incidentId"
                            params={{ incidentId: src.id }}
                            className="text-sm font-medium hover:text-accent transition-colors"
                          >
                            {src.title}
                          </Link>
                        </TableCell>
                        <TableCell><SeverityBadge severity={src.severity} /></TableCell>
                        <TableCell className="text-sm text-muted-foreground">{src.category || "\u2014"}</TableCell>
                        <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                          {formatRelativeTime(src.updated_at)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}

          {/* History section */}
          <Card>
            <CardHeader><CardTitle>History</CardTitle></CardHeader>
            <CardContent>
              {history && history.length > 0 ? (
                <div className="space-y-3">
                  {history.map((entry) => (
                    <div key={entry.id} className="flex items-start gap-3 text-sm border-b border-border pb-3 last:border-0 last:pb-0">
                      <div className="flex-shrink-0 mt-0.5">
                        <Badge variant={
                          entry.change_type === "created" ? "default" :
                          entry.change_type === "merged" ? "secondary" :
                          entry.change_type === "archived" ? "destructive" :
                          "outline"
                        }>
                          {entry.change_type}
                        </Badge>
                      </div>
                      <div className="flex-1 min-w-0">
                        <HistoryDiff changeType={entry.change_type} diff={entry.diff} />
                      </div>
                      <span className="text-muted-foreground whitespace-nowrap flex-shrink-0">
                        {formatRelativeTime(entry.created_at)}
                      </span>
                    </div>
                  ))}
                </div>
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
