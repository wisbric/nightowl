import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { formatRelativeTime } from "@/lib/utils";
import type { Runbook } from "@/types/api";

export function RunbookDetailPage() {
  const { runbookId } = useParams({ from: "/runbooks/$runbookId" });
  const isNew = runbookId === "new";
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(isNew);

  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [category, setCategory] = useState("");
  const [tags, setTags] = useState("");
  const [isTemplate, setIsTemplate] = useState(false);

  const { data: runbook, isLoading } = useQuery({
    queryKey: ["runbook", runbookId],
    queryFn: () => api.get<Runbook>(`/runbooks/${runbookId}`),
    enabled: !isNew,
  });

  useTitle(isNew ? "New Runbook" : runbook?.title ?? "Runbook");

  function startEditing() {
    if (runbook) {
      setTitle(runbook.title);
      setContent(runbook.content);
      setCategory(runbook.category);
      setTags((runbook.tags ?? []).join(", "));
      setIsTemplate(runbook.is_template);
    }
    setEditing(true);
  }

  const saveMutation = useMutation({
    mutationFn: () => {
      const body = {
        title,
        content,
        category,
        is_template: isTemplate,
        tags: tags.split(",").map((s) => s.trim()).filter(Boolean),
      };
      return isNew
        ? api.post<Runbook>("/runbooks", body)
        : api.put<Runbook>(`/runbooks/${runbookId}`, body);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["runbooks"] });
      if (!isNew) {
        queryClient.invalidateQueries({ queryKey: ["runbook", runbookId] });
        setEditing(false);
      }
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => api.delete(`/runbooks/${runbookId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["runbooks"] });
    },
  });

  if (!isNew && isLoading) return <p className="text-muted-foreground">Loading...</p>;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/runbooks" className="text-muted-foreground hover:text-foreground text-sm">&larr; Runbooks</Link>
      </div>

      {editing ? (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            saveMutation.mutate();
          }}
          className="space-y-6"
        >
          <Card>
            <CardHeader><CardTitle>{isNew ? "New Runbook" : "Edit Runbook"}</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              <div>
                <label className="text-sm font-medium">Title</label>
                <Input value={title} onChange={(e) => setTitle(e.target.value)} required />
              </div>
              <div>
                <label className="text-sm font-medium">Content</label>
                <Textarea value={content} onChange={(e) => setContent(e.target.value)} rows={12} className="font-mono text-sm" />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium">Category</label>
                  <Input value={category} onChange={(e) => setCategory(e.target.value)} />
                </div>
                <div>
                  <label className="text-sm font-medium">Tags (comma-separated)</label>
                  <Input value={tags} onChange={(e) => setTags(e.target.value)} />
                </div>
              </div>
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="is_template"
                  checked={isTemplate}
                  onChange={(e) => setIsTemplate(e.target.checked)}
                  className="h-4 w-4 rounded border-input"
                />
                <label htmlFor="is_template" className="text-sm font-medium">Template</label>
              </div>
              <div className="flex gap-2">
                <Button type="submit" disabled={saveMutation.isPending}>
                  {saveMutation.isPending ? "Saving..." : "Save"}
                </Button>
                {!isNew && (
                  <Button type="button" variant="outline" onClick={() => setEditing(false)}>
                    Cancel
                  </Button>
                )}
              </div>
            </CardContent>
          </Card>
        </form>
      ) : runbook ? (
        <>
          <div className="flex items-start justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold">{runbook.title}</h1>
              <div className="mt-2 flex items-center gap-3">
                {runbook.category && <Badge variant="outline">{runbook.category}</Badge>}
                {runbook.is_template && <Badge>Template</Badge>}
              </div>
            </div>
            <div className="flex gap-2">
              <Button onClick={startEditing}>Edit</Button>
              <Button
                variant="destructive"
                onClick={() => deleteMutation.mutate()}
                disabled={deleteMutation.isPending}
              >
                {deleteMutation.isPending ? "Deleting..." : "Delete"}
              </Button>
            </div>
          </div>

          {(runbook.tags ?? []).length > 0 && (
            <div className="flex flex-wrap gap-2">
              {runbook.tags.map((tag) => (
                <Badge key={tag} variant="secondary">{tag}</Badge>
              ))}
            </div>
          )}

          <Card>
            <CardHeader><CardTitle>Content</CardTitle></CardHeader>
            <CardContent>
              <pre className="whitespace-pre-wrap text-sm font-mono rounded-md bg-muted p-4">{runbook.content || "—"}</pre>
            </CardContent>
          </Card>

          <div className="text-xs text-muted-foreground">
            Created {formatRelativeTime(runbook.created_at)} &middot; Updated {formatRelativeTime(runbook.updated_at)}
          </div>
        </>
      ) : (
        <p className="text-muted-foreground">Runbook not found</p>
      )}
    </div>
  );
}
