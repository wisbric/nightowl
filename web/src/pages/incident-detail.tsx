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
import { formatRelativeTime } from "@/lib/utils";
import type { Incident } from "@/types/api";
import { useState } from "react";

interface IncidentForm {
  title: string;
  severity: string;
  category: string;
  symptoms: string;
  root_cause: string;
  solution: string;
  services: string;
  tags: string;
  error_patterns: string;
}

export function IncidentDetailPage() {
  const { incidentId } = useParams({ from: "/incidents/$incidentId" });
  const isNew = incidentId === "new";
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(isNew);

  const { data: incident, isLoading } = useQuery({
    queryKey: ["incident", incidentId],
    queryFn: () => api.get<Incident>(`/incidents/${incidentId}`),
    enabled: !isNew,
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

  if (!isNew && isLoading) return <p className="text-muted-foreground">Loading...</p>;

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
            <Button onClick={() => setEditing(true)}>Edit</Button>
          </div>

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
        </>
      ) : (
        <p className="text-muted-foreground">Incident not found</p>
      )}
    </div>
  );
}
