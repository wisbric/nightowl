import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRelativeTime } from "@/lib/utils";
import type { AlertGroupingRulesResponse, AlertGroupingRule, AlertGroupMatcher } from "@/types/api";
import { Layers, Plus, Pencil, Trash2, X } from "lucide-react";

const OP_OPTIONS = ["=", "!=", "=~", "!~"] as const;

interface RuleForm {
  name: string;
  description: string;
  position: number;
  is_enabled: boolean;
  matchers: AlertGroupMatcher[];
  group_by_input: string;
  group_by: string[];
}

function defaultForm(): RuleForm {
  return {
    name: "",
    description: "",
    position: 0,
    is_enabled: true,
    matchers: [],
    group_by_input: "",
    group_by: [],
  };
}

function ruleToForm(rule: AlertGroupingRule): RuleForm {
  return {
    name: rule.name,
    description: rule.description ?? "",
    position: rule.position,
    is_enabled: rule.is_enabled,
    matchers: [...rule.matchers],
    group_by_input: "",
    group_by: [...rule.group_by],
  };
}

function formToPayload(form: RuleForm) {
  return {
    name: form.name,
    description: form.description || undefined,
    position: form.position,
    is_enabled: form.is_enabled,
    matchers: form.matchers,
    group_by: form.group_by,
  };
}

export function AlertGroupRulesPage() {
  useTitle("Alert Grouping Rules");
  const queryClient = useQueryClient();

  const [editing, setEditing] = useState<string | null>(null); // null = not editing, "new" = creating, uuid = editing
  const [form, setForm] = useState<RuleForm>(defaultForm());
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["alert-grouping-rules"],
    queryFn: () => api.get<AlertGroupingRulesResponse>("/alert-groups/rules"),
  });
  const rules = data?.rules ?? [];

  const saveMutation = useMutation({
    mutationFn: (payload: ReturnType<typeof formToPayload>) =>
      editing === "new"
        ? api.post<AlertGroupingRule>("/alert-groups/rules", payload)
        : api.put<AlertGroupingRule>(`/alert-groups/rules/${editing}`, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alert-grouping-rules"] });
      setEditing(null);
      setForm(defaultForm());
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.delete(`/alert-groups/rules/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alert-grouping-rules"] });
      setConfirmDelete(null);
    },
  });

  function startCreate() {
    setForm(defaultForm());
    setEditing("new");
  }

  function startEdit(rule: AlertGroupingRule) {
    setForm(ruleToForm(rule));
    setEditing(rule.id);
  }

  function cancel() {
    setEditing(null);
    setForm(defaultForm());
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    saveMutation.mutate(formToPayload(form));
  }

  function addMatcher() {
    setForm((prev) => ({
      ...prev,
      matchers: [...prev.matchers, { key: "", op: "=", value: "" }],
    }));
  }

  function removeMatcher(index: number) {
    setForm((prev) => ({
      ...prev,
      matchers: prev.matchers.filter((_, i) => i !== index),
    }));
  }

  function updateMatcher(index: number, updates: Partial<AlertGroupMatcher>) {
    setForm((prev) => ({
      ...prev,
      matchers: prev.matchers.map((m, i) => (i === index ? { ...m, ...updates } : m)),
    }));
  }

  function addGroupByKey() {
    const key = form.group_by_input.trim();
    if (key && !form.group_by.includes(key)) {
      setForm((prev) => ({
        ...prev,
        group_by: [...prev.group_by, key],
        group_by_input: "",
      }));
    }
  }

  function removeGroupByKey(key: string) {
    setForm((prev) => ({
      ...prev,
      group_by: prev.group_by.filter((k) => k !== key),
    }));
  }

  function handleGroupByKeyDown(e: React.KeyboardEvent) {
    if (e.key === "Enter") {
      e.preventDefault();
      addGroupByKey();
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/alerts/groups" className="text-muted-foreground hover:text-foreground text-sm">&larr; Groups</Link>
      </div>

      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Grouping Rules</h1>
        {!editing && (
          <Button onClick={startCreate} size="sm">
            <Plus className="h-4 w-4 mr-1" />
            New Rule
          </Button>
        )}
      </div>

      {editing && (
        <form onSubmit={handleSubmit} className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>{editing === "new" ? "New Grouping Rule" : "Edit Rule"}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-3 gap-4">
                <div>
                  <label className="text-sm font-medium">Name</label>
                  <Input
                    value={form.name}
                    onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))}
                    placeholder="e.g. Production Alerts"
                    required
                  />
                </div>
                <div>
                  <label className="text-sm font-medium">Position (priority)</label>
                  <Input
                    type="number"
                    min={0}
                    value={form.position}
                    onChange={(e) => setForm((prev) => ({ ...prev, position: parseInt(e.target.value, 10) || 0 }))}
                  />
                </div>
                <div className="flex items-end">
                  <label className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={form.is_enabled}
                      onChange={(e) => setForm((prev) => ({ ...prev, is_enabled: e.target.checked }))}
                      className="rounded border-border"
                    />
                    Enabled
                  </label>
                </div>
              </div>
              <div>
                <label className="text-sm font-medium">Description</label>
                <Input
                  value={form.description}
                  onChange={(e) => setForm((prev) => ({ ...prev, description: e.target.value }))}
                  placeholder="Optional description"
                />
              </div>

              {/* Matchers */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-sm font-medium">Matchers (AND logic, empty = match all)</label>
                  <Button type="button" variant="outline" size="sm" onClick={addMatcher}>
                    + Add Matcher
                  </Button>
                </div>
                {form.matchers.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No matchers — this rule will match all alerts.</p>
                ) : (
                  <div className="space-y-2">
                    {form.matchers.map((m, i) => (
                      <div key={i} className="flex items-center gap-2">
                        <Input
                          value={m.key}
                          onChange={(e) => updateMatcher(i, { key: e.target.value })}
                          placeholder="Label key"
                          className="flex-1"
                        />
                        <select
                          value={m.op}
                          onChange={(e) => updateMatcher(i, { op: e.target.value })}
                          className="rounded-md border border-input bg-background px-3 py-2 text-sm"
                        >
                          {OP_OPTIONS.map((op) => (
                            <option key={op} value={op}>{op}</option>
                          ))}
                        </select>
                        <Input
                          value={m.value}
                          onChange={(e) => updateMatcher(i, { value: e.target.value })}
                          placeholder="Value"
                          className="flex-1"
                        />
                        <Button type="button" variant="ghost" size="sm" onClick={() => removeMatcher(i)}>
                          <X className="h-4 w-4" />
                        </Button>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* Group By */}
              <div>
                <label className="text-sm font-medium">Group By (label keys)</label>
                <div className="mt-1 flex flex-wrap gap-1 mb-2">
                  {form.group_by.map((key) => (
                    <Badge key={key} variant="secondary" className="text-xs gap-1">
                      {key}
                      <button type="button" onClick={() => removeGroupByKey(key)} className="ml-1 hover:text-destructive">
                        <X className="h-3 w-3" />
                      </button>
                    </Badge>
                  ))}
                </div>
                <div className="flex gap-2">
                  <Input
                    value={form.group_by_input}
                    onChange={(e) => setForm((prev) => ({ ...prev, group_by_input: e.target.value }))}
                    onKeyDown={handleGroupByKeyDown}
                    placeholder="Type a label key and press Enter"
                    className="flex-1"
                  />
                  <Button type="button" variant="outline" size="sm" onClick={addGroupByKey}>
                    Add
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>

          <div className="flex gap-2">
            <Button type="submit" disabled={saveMutation.isPending || form.group_by.length === 0}>
              {saveMutation.isPending ? "Saving..." : editing === "new" ? "Create Rule" : "Save Changes"}
            </Button>
            <Button type="button" variant="outline" onClick={cancel}>
              Cancel
            </Button>
          </div>
          {saveMutation.isError && (
            <p className="text-sm text-destructive">
              Error: {saveMutation.error instanceof Error ? saveMutation.error.message : "Failed to save"}
            </p>
          )}
        </form>
      )}

      <Card>
        <CardHeader><CardTitle>All Rules</CardTitle></CardHeader>
        <CardContent>
          {isLoading ? (
            <LoadingSpinner />
          ) : rules.length === 0 ? (
            <EmptyState
              title="No grouping rules"
              description="Create rules to automatically group related alerts."
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-16">Priority</TableHead>
                  <TableHead>Name</TableHead>
                  <TableHead>Matchers</TableHead>
                  <TableHead>Group By</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Updated</TableHead>
                  <TableHead className="w-24">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rules.map((rule) => (
                  <TableRow key={rule.id}>
                    <TableCell className="text-sm text-muted-foreground">{rule.position}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <Layers className="h-4 w-4 text-muted-foreground" />
                        <span className="font-medium text-sm">{rule.name}</span>
                      </div>
                      {rule.description && (
                        <p className="text-xs text-muted-foreground mt-0.5">{rule.description}</p>
                      )}
                    </TableCell>
                    <TableCell>
                      {rule.matchers.length === 0 ? (
                        <span className="text-xs text-muted-foreground">match all</span>
                      ) : (
                        <div className="flex flex-wrap gap-1">
                          {rule.matchers.map((m, i) => (
                            <Badge key={i} variant="outline" className="text-xs font-mono">
                              {m.key}{m.op}{m.value}
                            </Badge>
                          ))}
                        </div>
                      )}
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {rule.group_by.map((key) => (
                          <Badge key={key} variant="secondary" className="text-xs">{key}</Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={rule.is_enabled ? "default" : "outline"} className="text-xs">
                        {rule.is_enabled ? "Enabled" : "Disabled"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {formatRelativeTime(rule.updated_at)}
                    </TableCell>
                    <TableCell>
                      <div className="flex gap-1">
                        <Button variant="ghost" size="sm" onClick={() => startEdit(rule)}>
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        {confirmDelete === rule.id ? (
                          <div className="flex gap-1">
                            <Button
                              variant="destructive"
                              size="sm"
                              onClick={() => deleteMutation.mutate(rule.id)}
                              disabled={deleteMutation.isPending}
                            >
                              Confirm
                            </Button>
                            <Button variant="ghost" size="sm" onClick={() => setConfirmDelete(null)}>
                              <X className="h-3.5 w-3.5" />
                            </Button>
                          </div>
                        ) : (
                          <Button variant="ghost" size="sm" onClick={() => setConfirmDelete(rule.id)}>
                            <Trash2 className="h-3.5 w-3.5 text-destructive" />
                          </Button>
                        )}
                      </div>
                    </TableCell>
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
