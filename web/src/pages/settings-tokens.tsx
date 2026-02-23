import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Key, Plus, Copy, Check, Trash2, AlertTriangle } from "lucide-react";
import { api } from "@/lib/api";
import type { PATListResponse, PATCreateResponse } from "@/types/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogHeader, DialogTitle, DialogContent, DialogFooter } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";

export function SettingsTokensPage() {
  const queryClient = useQueryClient();
  const [showCreate, setShowCreate] = useState(false);
  const [newToken, setNewToken] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [name, setName] = useState("");
  const [expiresInDays, setExpiresInDays] = useState<string>("90");
  const [deleteId, setDeleteId] = useState<string | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["pat"],
    queryFn: () => api.get<PATListResponse>("/user/tokens"),
  });

  const createMutation = useMutation({
    mutationFn: (body: { name: string; expires_in_days?: number }) =>
      api.post<PATCreateResponse>("/user/tokens", body),
    onSuccess: (res) => {
      setNewToken(res.raw_token);
      setShowCreate(false);
      setName("");
      setExpiresInDays("90");
      queryClient.invalidateQueries({ queryKey: ["pat"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.delete(`/user/tokens/${id}`),
    onSuccess: () => {
      setDeleteId(null);
      queryClient.invalidateQueries({ queryKey: ["pat"] });
    },
  });

  function handleCreate() {
    const days = expiresInDays ? parseInt(expiresInDays, 10) : undefined;
    createMutation.mutate({
      name,
      expires_in_days: days && days > 0 ? days : undefined,
    });
  }

  function handleCopy() {
    if (newToken) {
      navigator.clipboard.writeText(newToken);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }

  const tokens = data?.tokens ?? [];

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Personal Access Tokens</h1>
          <p className="text-sm text-muted-foreground">
            Tokens authenticate API requests on your behalf. Treat them like passwords.
          </p>
        </div>
        <Button onClick={() => setShowCreate(true)} size="sm">
          <Plus className="h-4 w-4" />
          Generate token
        </Button>
      </div>

      {/* Newly created token banner — shown once */}
      {newToken && (
        <div className="rounded-lg border border-accent/30 bg-accent/5 p-4">
          <div className="flex items-start gap-3">
            <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-accent" />
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">
                Copy your token now — it won't be shown again.
              </p>
              <div className="mt-2 flex items-center gap-2">
                <code className="flex-1 truncate rounded bg-muted px-3 py-2 font-mono text-xs">
                  {newToken}
                </code>
                <Button variant="outline" size="sm" onClick={handleCopy}>
                  {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  {copied ? "Copied" : "Copy"}
                </Button>
              </div>
              <Button
                variant="ghost"
                size="sm"
                className="mt-2 text-xs text-muted-foreground"
                onClick={() => setNewToken(null)}
              >
                Dismiss
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Token list */}
      {isLoading ? (
        <div className="py-12 text-center text-sm text-muted-foreground">Loading tokens...</div>
      ) : tokens.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-12">
          <Key className="h-10 w-10 text-muted-foreground/40" />
          <p className="mt-3 text-sm text-muted-foreground">No tokens yet</p>
          <Button variant="outline" size="sm" className="mt-4" onClick={() => setShowCreate(true)}>
            Generate your first token
          </Button>
        </div>
      ) : (
        <div className="space-y-2">
          {tokens.map((token) => {
            const isExpired = token.expires_at && new Date(token.expires_at) < new Date();
            return (
              <div
                key={token.id}
                className="flex items-center justify-between rounded-lg border px-4 py-3"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-sm">{token.name}</span>
                    <code className="text-xs text-muted-foreground font-mono">{token.prefix}...</code>
                    {isExpired && <Badge variant="destructive">Expired</Badge>}
                  </div>
                  <div className="mt-1 flex gap-4 text-xs text-muted-foreground">
                    <span>Created {new Date(token.created_at).toLocaleDateString()}</span>
                    {token.expires_at && (
                      <span>
                        Expires {new Date(token.expires_at).toLocaleDateString()}
                      </span>
                    )}
                    {token.last_used_at ? (
                      <span>Last used {new Date(token.last_used_at).toLocaleDateString()}</span>
                    ) : (
                      <span>Never used</span>
                    )}
                  </div>
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  className="text-muted-foreground hover:text-destructive"
                  onClick={() => setDeleteId(token.id)}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            );
          })}
        </div>
      )}

      {/* Create dialog */}
      <Dialog open={showCreate} onClose={() => setShowCreate(false)}>
        <DialogHeader>
          <DialogTitle>Generate a new token</DialogTitle>
        </DialogHeader>
        <DialogContent>
          <div className="space-y-4">
            <div>
              <label className="text-sm font-medium">Name</label>
              <Input
                placeholder="e.g. CI pipeline, Terraform"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="mt-1"
                autoFocus
              />
            </div>
            <div>
              <label className="text-sm font-medium">Expiration (days)</label>
              <Input
                type="number"
                placeholder="90"
                min={0}
                value={expiresInDays}
                onChange={(e) => setExpiresInDays(e.target.value)}
                className="mt-1"
              />
              <p className="mt-1 text-xs text-muted-foreground">
                Set to 0 or leave empty for no expiration.
              </p>
            </div>
          </div>
        </DialogContent>
        <DialogFooter>
          <Button variant="outline" onClick={() => setShowCreate(false)}>
            Cancel
          </Button>
          <Button
            onClick={handleCreate}
            disabled={!name.trim() || createMutation.isPending}
          >
            {createMutation.isPending ? "Generating..." : "Generate token"}
          </Button>
        </DialogFooter>
      </Dialog>

      {/* Delete confirmation dialog */}
      <Dialog open={!!deleteId} onClose={() => setDeleteId(null)}>
        <DialogHeader>
          <DialogTitle>Revoke token</DialogTitle>
        </DialogHeader>
        <DialogContent>
          <p className="text-sm text-muted-foreground">
            This token will be permanently revoked. Any scripts or integrations using it will stop working immediately.
          </p>
        </DialogContent>
        <DialogFooter>
          <Button variant="outline" onClick={() => setDeleteId(null)}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            onClick={() => deleteId && deleteMutation.mutate(deleteId)}
            disabled={deleteMutation.isPending}
          >
            {deleteMutation.isPending ? "Revoking..." : "Revoke token"}
          </Button>
        </DialogFooter>
      </Dialog>
    </div>
  );
}
