import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Select } from "@/components/ui/select";
import { Dialog, DialogHeader, DialogTitle, DialogContent, DialogFooter } from "@/components/ui/dialog";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRelativeTime } from "@/lib/utils";
import type { ApiKeysResponse, ApiKeyCreateResponse } from "@/types/api";
import { Plus, Trash2, Copy, AlertTriangle } from "lucide-react";

interface KeyForm {
  description: string;
  role: string;
}

const emptyForm: KeyForm = {
  description: "",
  role: "engineer",
};

const ROLES = ["admin", "manager", "engineer", "readonly"] as const;

export function AdminApiKeysPage() {
  useTitle("API Keys");
  const queryClient = useQueryClient();

  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [form, setForm] = useState<KeyForm>(emptyForm);
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["api-keys"],
    queryFn: () => api.get<ApiKeysResponse>("/api-keys"),
  });

  const createMutation = useMutation({
    mutationFn: (data: KeyForm) => api.post<ApiKeyCreateResponse>("/api-keys", data),
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ["api-keys"] });
      setCreatedKey(result.raw_key);
      setForm(emptyForm);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.delete(`/api-keys/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["api-keys"] });
      setDeleteConfirmId(null);
    },
  });

  function openCreate() {
    setForm(emptyForm);
    setCreatedKey(null);
    setCopied(false);
    setShowCreateDialog(true);
  }

  function closeCreateDialog() {
    setShowCreateDialog(false);
    setCreatedKey(null);
    setCopied(false);
    setForm(emptyForm);
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    createMutation.mutate(form);
  }

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  const keys = data?.keys ?? [];

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/admin" className="text-muted-foreground hover:text-foreground text-sm">&larr; Admin</Link>
        <h1 className="text-2xl font-bold">API Keys</h1>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Keys ({data?.count ?? 0})</CardTitle>
            <Button size="sm" onClick={openCreate}>
              <Plus className="h-4 w-4 mr-1" />
              Create Key
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <LoadingSpinner />
          ) : keys.length === 0 ? (
            <EmptyState
              title="No API keys"
              description="Create an API key to integrate external tools."
              action={
                <Button size="sm" onClick={openCreate}>Create Key</Button>
              }
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Prefix</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Last Used</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="w-12"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {keys.map((key) => (
                  <TableRow key={key.id}>
                    <TableCell className="font-mono text-sm">{key.key_prefix}...</TableCell>
                    <TableCell className="text-sm">{key.description || "\u2014"}</TableCell>
                    <TableCell>
                      <Badge variant="outline" className="text-xs capitalize">{key.role}</Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {key.last_used ? formatRelativeTime(key.last_used) : "Never"}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {formatRelativeTime(key.created_at)}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => setDeleteConfirmId(key.id)}
                      >
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Create Key Dialog */}
      <Dialog open={showCreateDialog} onClose={closeCreateDialog}>
        {createdKey ? (
          <>
            <DialogHeader>
              <DialogTitle>API Key Created</DialogTitle>
            </DialogHeader>
            <DialogContent className="space-y-4">
              <div className="flex items-start gap-2 rounded-md border border-yellow-500/50 bg-yellow-500/10 p-3">
                <AlertTriangle className="h-5 w-5 text-yellow-500 shrink-0 mt-0.5" />
                <p className="text-sm text-yellow-200">
                  Copy this key now. You will not be able to see it again.
                </p>
              </div>
              <div className="relative">
                <pre className="rounded-md border bg-muted p-3 pr-10 text-sm font-mono break-all whitespace-pre-wrap">
                  {createdKey}
                </pre>
                <Button
                  variant="ghost"
                  size="icon"
                  className="absolute top-2 right-2"
                  onClick={() => copyToClipboard(createdKey)}
                >
                  <Copy className="h-4 w-4" />
                </Button>
              </div>
              {copied && (
                <p className="text-xs text-severity-ok">Copied to clipboard</p>
              )}
            </DialogContent>
            <DialogFooter>
              <Button onClick={closeCreateDialog}>Done</Button>
            </DialogFooter>
          </>
        ) : (
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>Create API Key</DialogTitle>
            </DialogHeader>
            <DialogContent className="space-y-4">
              <div>
                <label className="text-sm font-medium">Description</label>
                <Input
                  value={form.description}
                  onChange={(e) => setForm({ ...form, description: e.target.value })}
                  placeholder="e.g. CI/CD pipeline, monitoring integration"
                  required
                />
              </div>
              <div>
                <label className="text-sm font-medium">Role</label>
                <Select
                  value={form.role}
                  onChange={(e) => setForm({ ...form, role: e.target.value })}
                  required
                >
                  {ROLES.map((r) => (
                    <option key={r} value={r}>{r}</option>
                  ))}
                </Select>
              </div>
              {createMutation.isError && (
                <p className="text-sm text-destructive">
                  Error: {createMutation.error.message}
                </p>
              )}
            </DialogContent>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={closeCreateDialog}>Cancel</Button>
              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending ? "Creating..." : "Create Key"}
              </Button>
            </DialogFooter>
          </form>
        )}
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteConfirmId !== null} onClose={() => setDeleteConfirmId(null)}>
        <DialogHeader>
          <DialogTitle>Revoke API Key</DialogTitle>
        </DialogHeader>
        <DialogContent>
          <p className="text-sm text-muted-foreground">
            Are you sure you want to revoke this API key? Any integrations using this key will stop working immediately.
          </p>
          {deleteMutation.isError && (
            <p className="text-sm text-destructive mt-2">
              Error: {deleteMutation.error?.message}
            </p>
          )}
        </DialogContent>
        <DialogFooter>
          <Button variant="outline" onClick={() => setDeleteConfirmId(null)}>Cancel</Button>
          <Button
            variant="destructive"
            disabled={deleteMutation.isPending}
            onClick={() => {
              if (deleteConfirmId) deleteMutation.mutate(deleteConfirmId);
            }}
          >
            {deleteMutation.isPending ? "Revoking..." : "Revoke Key"}
          </Button>
        </DialogFooter>
      </Dialog>
    </div>
  );
}
