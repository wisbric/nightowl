import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { Dialog, DialogHeader, DialogTitle, DialogContent, DialogFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { UserSearchSelect } from "@/components/user-search-select";
import { formatRelativeTime } from "@/lib/utils";
import { TIMEZONES } from "@/lib/timezones";
import type {
  Roster,
  OnCallResponse,
  MembersResponse,
  OverridesResponse,
  ScheduleResponse,
  ScheduleEntry,
} from "@/types/api";
import { Calendar, Trash2, Lock, Unlock, Pencil, RefreshCw } from "lucide-react";

const DAYS = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];

interface RosterForm {
  name: string;
  timezone: string;
  handoff_time: string;
  handoff_day: string;
  schedule_weeks_ahead: string;
  max_consecutive_weeks: string;
  end_date: string;
}

interface OverrideForm {
  user_id: string;
  display_name: string;
  start_at: string;
  end_at: string;
  reason: string;
}

interface EditWeekForm {
  entry: ScheduleEntry;
  primary_user_id: string;
  secondary_user_id: string;
  notes: string;
  is_locked: boolean;
}

function formatWeekDate(dateStr: string): string {
  const d = new Date(dateStr + "T00:00:00");
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function isCurrentWeek(weekStart: string, weekEnd: string): boolean {
  const now = new Date();
  const start = new Date(weekStart + "T00:00:00");
  const end = new Date(weekEnd + "T23:59:59");
  return now >= start && now <= end;
}

function isPastWeek(weekEnd: string): boolean {
  const now = new Date();
  now.setHours(0, 0, 0, 0);
  const end = new Date(weekEnd + "T23:59:59");
  return end < now;
}

export function RosterDetailPage() {
  const { rosterId } = useParams({ from: "/rosters/$rosterId" });
  const isNew = rosterId === "new";
  const queryClient = useQueryClient();

  const [rosterForm, setRosterForm] = useState<RosterForm>({
    name: "",
    timezone: "UTC",
    handoff_time: "09:00",
    handoff_day: "1",
    schedule_weeks_ahead: "12",
    max_consecutive_weeks: "2",
    end_date: "",
  });

  const [overrideForm, setOverrideForm] = useState<OverrideForm>({
    user_id: "",
    display_name: "",
    start_at: "",
    end_at: "",
    reason: "",
  });

  const [editWeek, setEditWeek] = useState<EditWeekForm | null>(null);
  const [showRegenerate, setShowRegenerate] = useState(false);
  const [regenerateWeeks, setRegenerateWeeks] = useState("12");
  const [deactivateConfirm, setDeactivateConfirm] = useState<{ userId: string; name: string } | null>(null);

  const { data: roster, isLoading } = useQuery({
    queryKey: ["roster", rosterId],
    queryFn: () => api.get<Roster>(`/rosters/${rosterId}`),
    enabled: !isNew,
  });

  const { data: onCallData } = useQuery({
    queryKey: ["roster", rosterId, "oncall"],
    queryFn: () => api.get<OnCallResponse>(`/rosters/${rosterId}/oncall`),
    enabled: !isNew,
  });

  const { data: membersData } = useQuery({
    queryKey: ["roster", rosterId, "members"],
    queryFn: () => api.get<MembersResponse>(`/rosters/${rosterId}/members`),
    enabled: !isNew,
  });

  const { data: scheduleData } = useQuery({
    queryKey: ["roster", rosterId, "schedule"],
    queryFn: () => api.get<ScheduleResponse>(`/rosters/${rosterId}/schedule`),
    enabled: !isNew,
  });

  const { data: overridesData } = useQuery({
    queryKey: ["roster", rosterId, "overrides"],
    queryFn: () => api.get<OverridesResponse>(`/rosters/${rosterId}/overrides`),
    enabled: !isNew,
  });

  const members = membersData?.members ?? [];
  const activeMembers = members.filter((m) => m.is_active);
  const schedule = scheduleData?.schedule ?? [];
  const overrides = overridesData?.overrides ?? [];

  useTitle(isNew ? "New Roster" : roster?.name ?? "Roster");

  // --- Mutations ---

  const createRosterMutation = useMutation({
    mutationFn: (data: RosterForm) =>
      api.post<Roster>("/rosters", {
        name: data.name,
        timezone: data.timezone,
        handoff_time: data.handoff_time,
        handoff_day: parseInt(data.handoff_day, 10),
        schedule_weeks_ahead: parseInt(data.schedule_weeks_ahead, 10),
        max_consecutive_weeks: parseInt(data.max_consecutive_weeks, 10),
        end_date: data.end_date || null,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["rosters"] });
    },
  });

  const addMemberMutation = useMutation({
    mutationFn: (data: { user_id: string }) =>
      api.post(`/rosters/${rosterId}/members`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId] });
    },
  });

  const deactivateMemberMutation = useMutation({
    mutationFn: (userId: string) =>
      api.delete(`/rosters/${rosterId}/members/${userId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId] });
    },
  });

  const toggleMemberMutation = useMutation({
    mutationFn: ({ userId, isActive }: { userId: string; isActive: boolean }) =>
      api.put(`/rosters/${rosterId}/members/${userId}`, { is_active: isActive }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId] });
    },
  });

  const updateScheduleWeekMutation = useMutation({
    mutationFn: ({ weekStart, body }: { weekStart: string; body: Record<string, unknown> }) =>
      api.put(`/rosters/${rosterId}/schedule/${weekStart}`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId] });
      setEditWeek(null);
    },
  });

  const unlockWeekMutation = useMutation({
    mutationFn: (weekStart: string) =>
      api.delete(`/rosters/${rosterId}/schedule/${weekStart}/lock`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId] });
    },
  });

  const regenerateMutation = useMutation({
    mutationFn: (weeks: number) =>
      api.post(`/rosters/${rosterId}/schedule/generate`, {
        from: new Date().toISOString().slice(0, 10),
        weeks,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId] });
      setShowRegenerate(false);
    },
  });

  const addOverrideMutation = useMutation({
    mutationFn: (data: OverrideForm) =>
      api.post(`/rosters/${rosterId}/overrides`, {
        user_id: data.user_id,
        start_at: new Date(data.start_at).toISOString(),
        end_at: new Date(data.end_at).toISOString(),
        reason: data.reason || null,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId] });
      setOverrideForm({ user_id: "", display_name: "", start_at: "", end_at: "", reason: "" });
    },
  });

  const deleteOverrideMutation = useMutation({
    mutationFn: (overrideId: string) =>
      api.delete(`/rosters/${rosterId}/overrides/${overrideId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId] });
    },
  });

  // --- Create form ---

  if (isNew) {
    return (
      <div className="space-y-6">
        <div className="flex items-center gap-4">
          <Link to="/rosters" className="text-muted-foreground hover:text-foreground text-sm">&larr; Rosters</Link>
        </div>

        <h1 className="text-2xl font-bold">Create Roster</h1>

        <form onSubmit={(e) => { e.preventDefault(); createRosterMutation.mutate(rosterForm); }}>
          <Card>
            <CardHeader><CardTitle>Roster Details</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium">Name</label>
                  <Input value={rosterForm.name} onChange={(e) => setRosterForm({ ...rosterForm, name: e.target.value })} required />
                </div>
                <div>
                  <label className="text-sm font-medium">Timezone</label>
                  <Select value={rosterForm.timezone} onChange={(e) => setRosterForm({ ...rosterForm, timezone: e.target.value })} required>
                    {TIMEZONES.map((tz) => <option key={tz} value={tz}>{tz}</option>)}
                  </Select>
                </div>
              </div>
              <div className="grid grid-cols-3 gap-4">
                <div>
                  <label className="text-sm font-medium">Handoff Day</label>
                  <Select value={rosterForm.handoff_day} onChange={(e) => setRosterForm({ ...rosterForm, handoff_day: e.target.value })}>
                    {DAYS.map((d, i) => <option key={i} value={i}>{d}</option>)}
                  </Select>
                </div>
                <div>
                  <label className="text-sm font-medium">Handoff Time</label>
                  <Input type="time" value={rosterForm.handoff_time} onChange={(e) => setRosterForm({ ...rosterForm, handoff_time: e.target.value })} required />
                </div>
                <div>
                  <label className="text-sm font-medium">End Date</label>
                  <Input type="date" value={rosterForm.end_date} onChange={(e) => setRosterForm({ ...rosterForm, end_date: e.target.value })} />
                  <p className="text-xs text-muted-foreground mt-1">Leave empty for perpetual</p>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium">Weeks Ahead</label>
                  <Input type="number" min="1" max="52" value={rosterForm.schedule_weeks_ahead} onChange={(e) => setRosterForm({ ...rosterForm, schedule_weeks_ahead: e.target.value })} />
                  <p className="text-xs text-muted-foreground mt-1">How many future weeks to auto-generate</p>
                </div>
                <div>
                  <label className="text-sm font-medium">Max Consecutive Weeks</label>
                  <Input type="number" min="1" max="12" value={rosterForm.max_consecutive_weeks} onChange={(e) => setRosterForm({ ...rosterForm, max_consecutive_weeks: e.target.value })} />
                  <p className="text-xs text-muted-foreground mt-1">Max weeks same person is primary in a row</p>
                </div>
              </div>
              <div className="flex gap-2">
                <Button type="submit" disabled={createRosterMutation.isPending}>
                  {createRosterMutation.isPending ? "Creating..." : "Create Roster"}
                </Button>
                <Link to="/rosters"><Button type="button" variant="outline">Cancel</Button></Link>
              </div>
              {createRosterMutation.isSuccess && <p className="text-sm text-green-600">Roster created successfully.</p>}
              {createRosterMutation.isError && <p className="text-sm text-destructive">Error: {createRosterMutation.error?.message ?? "Failed"}</p>}
            </CardContent>
          </Card>
        </form>
      </div>
    );
  }

  if (isLoading) return <LoadingSpinner size="lg" />;
  if (!roster) return <p className="text-muted-foreground">Roster not found</p>;

  const lockedCount = schedule.filter((s) => s.is_locked && !isPastWeek(s.week_end)).length;
  const unlockedFuture = schedule.filter((s) => !s.is_locked && !isPastWeek(s.week_end)).length;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Link to="/rosters" className="text-muted-foreground hover:text-foreground text-sm">&larr; Rosters</Link>
      </div>

      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold">{roster.name}</h1>
            <Badge variant={roster.is_active ? "default" : "secondary"}>
              {roster.is_active ? "Active" : "Ended"}
            </Badge>
          </div>
          <p className="text-sm text-muted-foreground mt-1">
            {roster.timezone} &middot; Handoff: {DAYS[roster.handoff_day]} {roster.handoff_time}
          </p>
        </div>
        <a
          href={`/api/v1/rosters/${rosterId}/export.ics`}
          className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
          download
        >
          <Calendar className="h-4 w-4" />
          Export iCal
        </a>
      </div>

      {/* On-Call Now */}
      <Card>
        <CardHeader><CardTitle>On-Call Now</CardTitle></CardHeader>
        <CardContent>
          {onCallData ? (
            <div className="flex items-center gap-4 flex-wrap">
              <div className="flex items-center gap-2">
                <Badge className="bg-green-600/20 text-green-400 border-green-600/30">Primary</Badge>
                <span className="text-sm font-medium">
                  {onCallData.primary?.display_name ?? "None"}
                </span>
              </div>
              <div className="flex items-center gap-2">
                <Badge variant="outline">Secondary</Badge>
                <span className="text-sm text-muted-foreground">
                  {onCallData.secondary?.display_name ?? "None"}
                </span>
              </div>
              <Badge variant="secondary" className="ml-auto">
                Source: {onCallData.source}
                {onCallData.week_start && ` (Week of ${formatWeekDate(onCallData.week_start)})`}
              </Badge>
              {onCallData.active_override && (
                <Badge variant="outline" className="text-amber-400 border-amber-400/30">
                  Override: {onCallData.active_override.display_name}
                </Badge>
              )}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Loading...</p>
          )}
        </CardContent>
      </Card>

      {/* Schedule */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Schedule ({schedule.length} weeks)</CardTitle>
            <Button size="sm" variant="outline" onClick={() => setShowRegenerate(true)}>
              <RefreshCw className="h-4 w-4 mr-1" />
              Regenerate
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {schedule.length > 0 ? (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Week of</TableHead>
                    <TableHead>Primary</TableHead>
                    <TableHead>Secondary</TableHead>
                    <TableHead className="w-16">Lock</TableHead>
                    <TableHead className="w-16">Edit</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {schedule.map((entry) => {
                    const past = isPastWeek(entry.week_end);
                    const current = isCurrentWeek(entry.week_start, entry.week_end);
                    return (
                      <TableRow
                        key={entry.id}
                        className={past ? "opacity-50" : current ? "bg-primary/5 border-l-2 border-l-primary" : ""}
                      >
                        <TableCell className="text-sm font-medium">
                          <span className="flex items-center gap-1.5">
                            {current && <span className="h-2 w-2 rounded-full bg-primary inline-block" />}
                            {formatWeekDate(entry.week_start)}
                          </span>
                        </TableCell>
                        <TableCell className="text-sm">
                          {entry.primary_display_name || (entry.primary_user_id ? entry.primary_user_id.slice(0, 8) : <span className="text-muted-foreground">Unassigned</span>)}
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {entry.secondary_display_name || (entry.secondary_user_id ? entry.secondary_user_id.slice(0, 8) : "\u2014")}
                        </TableCell>
                        <TableCell>
                          {entry.is_locked ? (
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-7 w-7"
                              title="Unlock week"
                              onClick={() => unlockWeekMutation.mutate(entry.week_start)}
                              disabled={past}
                            >
                              <Lock className="h-3.5 w-3.5 text-amber-400" />
                            </Button>
                          ) : !past ? (
                            <Unlock className="h-3.5 w-3.5 text-muted-foreground/40" />
                          ) : null}
                        </TableCell>
                        <TableCell>
                          {!past && (
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-7 w-7"
                              onClick={() => setEditWeek({
                                entry,
                                primary_user_id: entry.primary_user_id ?? "",
                                secondary_user_id: entry.secondary_user_id ?? "",
                                notes: entry.notes ?? "",
                                is_locked: true,
                              })}
                            >
                              <Pencil className="h-3.5 w-3.5" />
                            </Button>
                          )}
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
              <p className="text-xs text-muted-foreground mt-3">
                <span className="inline-flex items-center gap-1"><span className="h-2 w-2 rounded-full bg-primary inline-block" /> Current week</span>
                <span className="mx-2">&middot;</span>
                <span className="inline-flex items-center gap-1"><Lock className="h-3 w-3 text-amber-400" /> Locked (won't auto-regenerate)</span>
              </p>
            </>
          ) : (
            <p className="text-sm text-muted-foreground">No schedule generated. Add members and click Regenerate.</p>
          )}
        </CardContent>
      </Card>

      {/* Members */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Members ({activeMembers.length} active)</CardTitle>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {members.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Primary Weeks</TableHead>
                  <TableHead>Joined</TableHead>
                  <TableHead className="w-32">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {members.map((m) => (
                  <TableRow key={m.id} className={!m.is_active ? "opacity-60" : ""}>
                    <TableCell className="text-sm font-medium">{m.display_name}</TableCell>
                    <TableCell>
                      {m.is_active ? (
                        <Badge className="bg-green-600/20 text-green-400 border-green-600/30">Active</Badge>
                      ) : (
                        <Badge variant="secondary">Inactive</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-sm">{m.primary_weeks_served}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">{formatRelativeTime(m.joined_at)}</TableCell>
                    <TableCell>
                      {m.is_active ? (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="text-destructive hover:text-destructive"
                          onClick={() => setDeactivateConfirm({ userId: m.user_id, name: m.display_name })}
                        >
                          Deactivate
                        </Button>
                      ) : (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => toggleMemberMutation.mutate({ userId: m.user_id, isActive: true })}
                          disabled={toggleMemberMutation.isPending}
                        >
                          Activate
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <p className="text-sm text-muted-foreground">No members</p>
          )}

          <div className="space-y-2 border-t pt-4">
            <p className="text-xs font-medium text-muted-foreground">Add Member</p>
            <UserSearchSelect
              excludeUserIds={members.map((m) => m.user_id)}
              rosterTimezone={roster.timezone}
              placeholder="Search users to add..."
              onSelect={(user) => addMemberMutation.mutate({ user_id: user.id })}
            />
            {addMemberMutation.isError && (
              <p className="text-sm text-destructive">Failed to add member: {addMemberMutation.error?.message ?? "Unknown error"}</p>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Overrides */}
      <Card>
        <CardHeader><CardTitle>Overrides ({overrides.length})</CardTitle></CardHeader>
        <CardContent className="space-y-4">
          {overrides.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Covering</TableHead>
                  <TableHead>Start</TableHead>
                  <TableHead>End</TableHead>
                  <TableHead>Reason</TableHead>
                  <TableHead className="w-12"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {overrides.map((o) => (
                  <TableRow key={o.id}>
                    <TableCell className="text-sm font-medium">{o.display_name}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">{new Date(o.start_at).toLocaleString()}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">{new Date(o.end_at).toLocaleString()}</TableCell>
                    <TableCell className="text-sm">{o.reason || "\u2014"}</TableCell>
                    <TableCell>
                      <Button variant="ghost" size="icon" onClick={() => deleteOverrideMutation.mutate(o.id)} disabled={deleteOverrideMutation.isPending}>
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <p className="text-sm text-muted-foreground">No overrides</p>
          )}

          <form className="space-y-2 border-t pt-4" onSubmit={(e) => { e.preventDefault(); addOverrideMutation.mutate(overrideForm); }}>
            <p className="text-xs font-medium text-muted-foreground">Add Override</p>
            <UserSearchSelect
              placeholder="Search user for override..."
              rosterTimezone={roster.timezone}
              onSelect={(user) => setOverrideForm({ ...overrideForm, user_id: user.id, display_name: user.display_name })}
            />
            {overrideForm.display_name && <p className="text-sm">Selected: <span className="font-medium">{overrideForm.display_name}</span></p>}
            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="text-xs text-muted-foreground">Start</label>
                <Input type="datetime-local" value={overrideForm.start_at} onChange={(e) => setOverrideForm({ ...overrideForm, start_at: e.target.value })} required />
              </div>
              <div>
                <label className="text-xs text-muted-foreground">End</label>
                <Input type="datetime-local" value={overrideForm.end_at} onChange={(e) => setOverrideForm({ ...overrideForm, end_at: e.target.value })} required />
              </div>
            </div>
            <Input placeholder="Reason (optional)" value={overrideForm.reason} onChange={(e) => setOverrideForm({ ...overrideForm, reason: e.target.value })} />
            <Button type="submit" size="sm" disabled={addOverrideMutation.isPending || !overrideForm.user_id}>
              {addOverrideMutation.isPending ? "Adding..." : "Add Override"}
            </Button>
          </form>
        </CardContent>
      </Card>

      {/* Footer info */}
      <div className="text-xs text-muted-foreground">
        Created {formatRelativeTime(roster.created_at)} &middot; Updated {formatRelativeTime(roster.updated_at)}
        {roster.end_date && <> &middot; End date: {roster.end_date}</>}
      </div>

      {/* Edit Week Dialog */}
      <Dialog open={editWeek !== null} onClose={() => setEditWeek(null)}>
        <DialogHeader>
          <DialogTitle>
            Edit Schedule: Week of {editWeek ? formatWeekDate(editWeek.entry.week_start) : ""}
          </DialogTitle>
        </DialogHeader>
        <DialogContent className="space-y-4">
          <div>
            <label className="text-sm font-medium">Primary</label>
            <Select
              value={editWeek?.primary_user_id ?? ""}
              onChange={(e) => editWeek && setEditWeek({ ...editWeek, primary_user_id: e.target.value })}
            >
              <option value="">Unassigned</option>
              {activeMembers.map((m) => (
                <option key={m.user_id} value={m.user_id} disabled={m.user_id === editWeek?.secondary_user_id}>
                  {m.display_name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <label className="text-sm font-medium">Secondary</label>
            <Select
              value={editWeek?.secondary_user_id ?? ""}
              onChange={(e) => editWeek && setEditWeek({ ...editWeek, secondary_user_id: e.target.value })}
            >
              <option value="">None</option>
              {activeMembers.map((m) => (
                <option key={m.user_id} value={m.user_id} disabled={m.user_id === editWeek?.primary_user_id}>
                  {m.display_name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <label className="text-sm font-medium">Notes</label>
            <Input
              placeholder="Optional context..."
              value={editWeek?.notes ?? ""}
              onChange={(e) => editWeek && setEditWeek({ ...editWeek, notes: e.target.value })}
            />
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={editWeek?.is_locked ?? true}
              onChange={(e) => editWeek && setEditWeek({ ...editWeek, is_locked: e.target.checked })}
              className="rounded border-input"
            />
            Lock this week (prevents auto-regeneration)
          </label>
        </DialogContent>
        <DialogFooter>
          <Button variant="outline" onClick={() => setEditWeek(null)}>Cancel</Button>
          <Button
            disabled={updateScheduleWeekMutation.isPending}
            onClick={() => {
              if (!editWeek) return;
              updateScheduleWeekMutation.mutate({
                weekStart: editWeek.entry.week_start,
                body: {
                  primary_user_id: editWeek.primary_user_id || null,
                  secondary_user_id: editWeek.secondary_user_id || null,
                  notes: editWeek.notes || null,
                },
              });
            }}
          >
            {updateScheduleWeekMutation.isPending ? "Saving..." : "Save"}
          </Button>
        </DialogFooter>
      </Dialog>

      {/* Regenerate Dialog */}
      <Dialog open={showRegenerate} onClose={() => setShowRegenerate(false)}>
        <DialogHeader>
          <DialogTitle>Regenerate Schedule</DialogTitle>
        </DialogHeader>
        <DialogContent className="space-y-3">
          <p className="text-sm text-muted-foreground">
            This will reassign primary/secondary for all <strong>unlocked</strong> future weeks based on fair rotation.
          </p>
          {lockedCount > 0 && (
            <p className="text-sm flex items-center gap-1">
              <Lock className="h-3.5 w-3.5 text-amber-400" />
              {lockedCount} locked week{lockedCount !== 1 ? "s" : ""} will not be changed
            </p>
          )}
          <p className="text-sm">{unlockedFuture} week{unlockedFuture !== 1 ? "s" : ""} will be regenerated</p>
          <div>
            <label className="text-sm font-medium">Generate weeks from today</label>
            <Input
              type="number"
              min="1"
              max="52"
              value={regenerateWeeks}
              onChange={(e) => setRegenerateWeeks(e.target.value)}
            />
          </div>
        </DialogContent>
        <DialogFooter>
          <Button variant="outline" onClick={() => setShowRegenerate(false)}>Cancel</Button>
          <Button
            disabled={regenerateMutation.isPending}
            onClick={() => regenerateMutation.mutate(parseInt(regenerateWeeks, 10))}
          >
            {regenerateMutation.isPending ? "Regenerating..." : "Regenerate"}
          </Button>
        </DialogFooter>
      </Dialog>

      {/* Deactivate Member Confirmation */}
      <Dialog open={deactivateConfirm !== null} onClose={() => setDeactivateConfirm(null)}>
        <DialogHeader>
          <DialogTitle>Deactivate Member</DialogTitle>
        </DialogHeader>
        <DialogContent>
          <p className="text-sm text-muted-foreground">
            Deactivate <span className="font-medium text-foreground">{deactivateConfirm?.name}</span> from this roster?
            They will no longer be scheduled for on-call. Future unlocked weeks will be regenerated.
          </p>
        </DialogContent>
        <DialogFooter>
          <Button variant="outline" onClick={() => setDeactivateConfirm(null)}>Cancel</Button>
          <Button
            variant="destructive"
            disabled={deactivateMemberMutation.isPending}
            onClick={() => {
              if (deactivateConfirm) {
                deactivateMemberMutation.mutate(deactivateConfirm.userId, {
                  onSuccess: () => setDeactivateConfirm(null),
                });
              }
            }}
          >
            {deactivateMemberMutation.isPending ? "Deactivating..." : "Deactivate"}
          </Button>
        </DialogFooter>
      </Dialog>
    </div>
  );
}
