import { useQuery } from "@tanstack/react-query";
import { useParams, Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { formatRelativeTime } from "@/lib/utils";
import type { BookOwlRunbookDetail } from "@/types/api";
import { ExternalLink } from "lucide-react";

export function RunbookDetailPage() {
  const { runbookId } = useParams({ strict: false }) as { runbookId: string };

  const { data: runbook, isLoading } = useQuery({
    queryKey: ["bookowl-runbook", runbookId],
    queryFn: () => api.get<BookOwlRunbookDetail>(`/bookowl/runbooks/${runbookId}`),
  });

  useTitle(runbook?.title ?? "Runbook");

  if (isLoading) return <LoadingSpinner size="lg" />;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/runbooks" className="text-muted-foreground hover:text-foreground text-sm">&larr; Runbooks</Link>
      </div>

      {runbook ? (
        <>
          <div className="flex items-start justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold">{runbook.title}</h1>
              {(runbook.tags ?? []).length > 0 && (
                <div className="mt-2 flex flex-wrap gap-2">
                  {runbook.tags.map((tag) => (
                    <Badge key={tag} variant="secondary">{tag}</Badge>
                  ))}
                </div>
              )}
            </div>
            <a href={runbook.url} target="_blank" rel="noopener noreferrer">
              <Button variant="outline">
                <ExternalLink className="h-3.5 w-3.5 mr-1" />
                Open in BookOwl
              </Button>
            </a>
          </div>

          <Card>
            <CardHeader><CardTitle>Content</CardTitle></CardHeader>
            <CardContent>
              {runbook.content_html ? (
                <div
                  className="prose prose-invert prose-sm max-w-none"
                  dangerouslySetInnerHTML={{ __html: runbook.content_html }}
                />
              ) : (
                <pre className="whitespace-pre-wrap text-sm font-mono rounded-md bg-muted p-4">
                  {runbook.content_text || "\u2014"}
                </pre>
              )}
            </CardContent>
          </Card>

          <div className="text-xs text-muted-foreground">
            Updated {formatRelativeTime(runbook.updated_at)}
          </div>
        </>
      ) : (
        <p className="text-muted-foreground">Runbook not found</p>
      )}
    </div>
  );
}
