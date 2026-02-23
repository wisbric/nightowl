import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { formatRelativeTime } from "@/lib/utils";
import type { EscalationPolicy, DryRunResponse } from "@/types/api";
import { useState } from "react";

const NOTIFY_VIA_OPTIONS = ["slack_dm", "slack_channel", "phone", "sms", "email"] as const;

interface TierForm {
  tier: number;
  timeout_minutes: number;
  notify_via: string[];
  targets: string;
}

interface PolicyForm {
  name: string;
  description: string;
  repeat_count: number;
  tiers: TierForm[];
}

function emptyTier(tierNumber: number): TierForm {
  return { tier: tierNumber, timeout_minutes: 5, notify_via: [], targets: "" };
}

function policyToForm(policy: EscalationPolicy): PolicyForm {
  return {
    name: policy.name,
    description: policy.description ?? "",
    repeat_count: policy.repeat_count ?? 0,
    tiers: policy.tiers.map((t) => ({
      tier: t.tier,
      timeout_minutes: t.timeout_minutes,
      notify_via: [...t.notify_via],
      targets: t.targets.join(", "),
    })),
  };
}

function formToPayload(form: PolicyForm) {
  return {
    name: form.name,
    description: form.description || undefined,
    repeat_count: form.repeat_count,
    tiers: form.tiers.map((t, i) => ({
      tier: i + 1,
      timeout_minutes: t.timeout_minutes,
      notify_via: t.notify_via,
      targets: t.targets
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean),
    })),
  };
}

function defaultForm(): PolicyForm {
  return {
    name: "",
    description: "",
    repeat_count: 0,
    tiers: [emptyTier(1)],
  };
}

export function EscalationDetailPage() {
  const { policyId } = useParams({ strict: false }) as { policyId: string };
  const isNew = policyId === "new";
  const queryClient = useQueryClient();

  const [editing, setEditing] = useState(isNew);
  const [form, setForm] = useState<PolicyForm>(defaultForm());
  const [dryRunResult, setDryRunResult] = useState<DryRunResponse | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const { data: policy, isLoading } = useQuery({
    queryKey: ["escalation-policy", policyId],
    queryFn: () => api.get<EscalationPolicy>(`/escalation-policies/${policyId}`),
    enabled: !isNew,
  });

  useTitle(isNew ? "New Escalation Policy" : policy?.name ?? "Escalation Policy");

  const saveMutation = useMutation({
    mutationFn: (payload: ReturnType<typeof formToPayload>) =>
      isNew
        ? api.post<EscalationPolicy>("/escalation-policies", payload)
        : api.put<EscalationPolicy>(`/escalation-policies/${policyId}`, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["escalation-policies"] });
      if (!isNew) {
        queryClient.invalidateQueries({ queryKey: ["escalation-policy", policyId] });
        setEditing(false);
      }
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => api.delete(`/escalation-policies/${policyId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["escalation-policies"] });
    },
  });

  const dryRunMutation = useMutation({
    mutationFn: () =>
      api.post<DryRunResponse>(`/escalation-policies/${policyId}/dry-run`, {}),
    onSuccess: (data) => setDryRunResult(data),
  });

  function startEditing() {
    if (policy) {
      setForm(policyToForm(policy));
    }
    setEditing(true);
  }

  function cancelEditing() {
    setEditing(false);
    setForm(defaultForm());
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    saveMutation.mutate(formToPayload(form));
  }

  function updateTier(index: number, updates: Partial<TierForm>) {
    setForm((prev) => ({
      ...prev,
      tiers: prev.tiers.map((t, i) => (i === index ? { ...t, ...updates } : t)),
    }));
  }

  function addTier() {
    setForm((prev) => ({
      ...prev,
      tiers: [...prev.tiers, emptyTier(prev.tiers.length + 1)],
    }));
  }

  function removeTier(index: number) {
    setForm((prev) => ({
      ...prev,
      tiers: prev.tiers
        .filter((_, i) => i !== index)
        .map((t, i) => ({ ...t, tier: i + 1 })),
    }));
  }

  function toggleNotifyVia(tierIndex: number, channel: string) {
    setForm((prev) => ({
      ...prev,
      tiers: prev.tiers.map((t, i) => {
        if (i !== tierIndex) return t;
        const has = t.notify_via.includes(channel);
        return {
          ...t,
          notify_via: has
            ? t.notify_via.filter((v) => v !== channel)
            : [...t.notify_via, channel],
        };
      }),
    }));
  }

  if (!isNew && isLoading) return <LoadingSpinner size="lg" />;

  if (deleteMutation.isSuccess) {
    return (
      <div className="space-y-6">
        <div className="flex items-center gap-4">
          <Link to="/escalation" className="text-muted-foreground hover:text-foreground text-sm">&larr; Policies</Link>
        </div>
        <Card>
          <CardContent className="py-8 text-center">
            <p className="text-muted-foreground">Policy deleted successfully.</p>
            <Link to="/escalation" className="mt-4 inline-block text-sm text-primary hover:underline">
              Back to policies
            </Link>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/escalation" className="text-muted-foreground hover:text-foreground text-sm">&larr; Policies</Link>
      </div>

      {editing ? (
        <form onSubmit={handleSubmit} className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>{isNew ? "New Escalation Policy" : "Edit Policy"}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium">Name</label>
                  <Input
                    value={form.name}
                    onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))}
                    placeholder="e.g. Critical Alert Escalation"
                    required
                  />
                </div>
                <div>
                  <label className="text-sm font-medium">Repeat Count</label>
                  <Input
                    type="number"
                    min={0}
                    value={form.repeat_count}
                    onChange={(e) =>
                      setForm((prev) => ({ ...prev, repeat_count: parseInt(e.target.value, 10) || 0 }))
                    }
                  />
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
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>Tiers</CardTitle>
                <Button type="button" variant="outline" size="sm" onClick={addTier}>
                  + Add Tier
                </Button>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              {form.tiers.map((tier, index) => (
                <div key={index} className="rounded-md border p-4 space-y-3">
                  <div className="flex items-center justify-between">
                    <Badge variant="outline">Tier {index + 1}</Badge>
                    {form.tiers.length > 1 && (
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="text-destructive hover:text-destructive"
                        onClick={() => removeTier(index)}
                      >
                        Remove
                      </Button>
                    )}
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="text-sm font-medium">Timeout (minutes)</label>
                      <Input
                        type="number"
                        min={1}
                        value={tier.timeout_minutes}
                        onChange={(e) =>
                          updateTier(index, { timeout_minutes: parseInt(e.target.value, 10) || 1 })
                        }
                        required
                      />
                    </div>
                    <div>
                      <label className="text-sm font-medium">Targets (comma-separated)</label>
                      <Input
                        value={tier.targets}
                        onChange={(e) => updateTier(index, { targets: e.target.value })}
                        placeholder="e.g. @oncall-team, #alerts-channel"
                      />
                    </div>
                  </div>
                  <div>
                    <label className="text-sm font-medium">Notification Channels</label>
                    <div className="mt-1 flex flex-wrap gap-2">
                      {NOTIFY_VIA_OPTIONS.map((channel) => {
                        const isSelected = tier.notify_via.includes(channel);
                        return (
                          <button
                            key={channel}
                            type="button"
                            onClick={() => toggleNotifyVia(index, channel)}
                            className={
                              isSelected
                                ? "inline-flex items-center rounded-md border border-primary bg-primary/10 px-2.5 py-0.5 text-xs font-medium text-primary transition-colors"
                                : "inline-flex items-center rounded-md border border-input bg-background px-2.5 py-0.5 text-xs font-medium text-muted-foreground transition-colors hover:bg-muted"
                            }
                          >
                            {channel.replace("_", " ")}
                          </button>
                        );
                      })}
                    </div>
                  </div>
                </div>
              ))}
            </CardContent>
          </Card>

          <div className="flex gap-2">
            <Button type="submit" disabled={saveMutation.isPending}>
              {saveMutation.isPending ? "Saving..." : isNew ? "Create Policy" : "Save Changes"}
            </Button>
            {!isNew && (
              <Button type="button" variant="outline" onClick={cancelEditing}>
                Cancel
              </Button>
            )}
          </div>
          {saveMutation.isError && (
            <p className="text-sm text-destructive">
              Error: {saveMutation.error instanceof Error ? saveMutation.error.message : "Failed to save"}
            </p>
          )}
        </form>
      ) : policy ? (
        <>
          <div className="flex items-start justify-between">
            <div>
              <h1 className="text-2xl font-bold">{policy.name}</h1>
              {policy.description && (
                <p className="mt-1 text-sm text-muted-foreground">{policy.description}</p>
              )}
              <div className="mt-2 flex items-center gap-3 text-sm text-muted-foreground">
                {policy.repeat_count !== undefined && policy.repeat_count > 0 && (
                  <span>Repeats: {policy.repeat_count}</span>
                )}
                <span>Created {formatRelativeTime(policy.created_at)}</span>
                <span>Updated {formatRelativeTime(policy.updated_at)}</span>
              </div>
            </div>
            <div className="flex gap-2">
              <Button onClick={startEditing}>Edit</Button>
              <Button
                onClick={() => dryRunMutation.mutate()}
                disabled={dryRunMutation.isPending}
                variant="outline"
              >
                {dryRunMutation.isPending ? "Running..." : "Dry Run"}
              </Button>
              {!confirmDelete ? (
                <Button variant="destructive" onClick={() => setConfirmDelete(true)}>
                  Delete
                </Button>
              ) : (
                <div className="flex gap-1">
                  <Button
                    variant="destructive"
                    onClick={() => deleteMutation.mutate()}
                    disabled={deleteMutation.isPending}
                  >
                    {deleteMutation.isPending ? "Deleting..." : "Confirm Delete"}
                  </Button>
                  <Button variant="outline" onClick={() => setConfirmDelete(false)}>
                    Cancel
                  </Button>
                </div>
              )}
            </div>
          </div>

          <Card>
            <CardHeader>
              <CardTitle>Escalation Tiers</CardTitle>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Level</TableHead>
                    <TableHead>Timeout</TableHead>
                    <TableHead>Notification</TableHead>
                    <TableHead>Targets</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {policy.tiers?.map((tier) => (
                    <TableRow key={tier.tier}>
                      <TableCell>
                        <Badge variant="outline">L{tier.tier}</Badge>
                      </TableCell>
                      <TableCell className="text-sm">{tier.timeout_minutes} minutes</TableCell>
                      <TableCell className="text-sm capitalize">
                        {tier.notify_via.join(", ")}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {tier.targets.map((t) => (
                            <Badge key={t} variant="secondary" className="text-xs">
                              {t}
                            </Badge>
                          ))}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          {deleteMutation.isError && (
            <p className="text-sm text-destructive">
              Error: {deleteMutation.error instanceof Error ? deleteMutation.error.message : "Failed to delete"}
            </p>
          )}

          {dryRunMutation.isError && (
            <p className="text-sm text-destructive">
              Error: {dryRunMutation.error instanceof Error ? dryRunMutation.error.message : "Dry run failed"}
            </p>
          )}

          {dryRunResult && (
            <Card>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <CardTitle>Dry Run Result</CardTitle>
                  <span className="text-sm text-muted-foreground">
                    Total time: {dryRunResult.total_time_minutes} minutes
                  </span>
                </div>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Tier</TableHead>
                      <TableHead>At</TableHead>
                      <TableHead>Timeout</TableHead>
                      <TableHead>Action</TableHead>
                      <TableHead>Channels</TableHead>
                      <TableHead>Targets</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {dryRunResult.steps.map((step, i) => (
                      <TableRow key={i}>
                        <TableCell>
                          <Badge variant="outline">L{step.tier}</Badge>
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {step.cumulative_minutes}m
                        </TableCell>
                        <TableCell className="text-sm">
                          {step.timeout_minutes}m
                        </TableCell>
                        <TableCell className="text-sm">{step.action}</TableCell>
                        <TableCell className="text-sm capitalize">
                          {step.notify_via.join(", ")}
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1">
                            {step.targets.map((t) => (
                              <Badge key={t} variant="secondary" className="text-xs">
                                {t}
                              </Badge>
                            ))}
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}
        </>
      ) : (
        <p className="text-muted-foreground">Policy not found</p>
      )}
    </div>
  );
}
