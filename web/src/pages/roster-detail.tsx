import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { formatRelativeTime } from "@/lib/utils";
import type { Roster, OnCallResponse, MembersResponse, OverridesResponse } from "@/types/api";
import { Calendar, Trash2 } from "lucide-react";

interface RosterForm {
  name: string;
  timezone: string;
  rotation_type: string;
  rotation_length: string;
  handoff_time: string;
  start_date: string;
}

interface MemberForm {
  user_id: string;
  display_name: string;
  position: string;
}

interface OverrideForm {
  user_id: string;
  display_name: string;
  start_time: string;
  end_time: string;
  reason: string;
}

export function RosterDetailPage() {
  const { rosterId } = useParams({ from: "/rosters/$rosterId" });
  const isNew = rosterId === "new";
  const queryClient = useQueryClient();

  const [rosterForm, setRosterForm] = useState<RosterForm>({
    name: "",
    timezone: "UTC",
    rotation_type: "daily",
    rotation_length: "1",
    handoff_time: "09:00",
    start_date: new Date().toISOString().slice(0, 10),
  });

  const [memberForm, setMemberForm] = useState<MemberForm>({
    user_id: "",
    display_name: "",
    position: "0",
  });

  const [overrideForm, setOverrideForm] = useState<OverrideForm>({
    user_id: "",
    display_name: "",
    start_time: "",
    end_time: "",
    reason: "",
  });

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

  const { data: overridesData } = useQuery({
    queryKey: ["roster", rosterId, "overrides"],
    queryFn: () => api.get<OverridesResponse>(`/rosters/${rosterId}/overrides`),
    enabled: !isNew,
  });

  const members = membersData?.members ?? [];
  const overrides = overridesData?.overrides ?? [];

  useTitle(isNew ? "New Roster" : roster?.name ?? "Roster");

  const createRosterMutation = useMutation({
    mutationFn: (data: RosterForm) =>
      api.post<Roster>("/rosters", {
        ...data,
        rotation_length: parseInt(data.rotation_length, 10),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["rosters"] });
    },
  });

  const addMemberMutation = useMutation({
    mutationFn: (data: MemberForm) =>
      api.post(`/rosters/${rosterId}/members`, {
        user_id: data.user_id,
        display_name: data.display_name,
        position: parseInt(data.position, 10),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId, "members"] });
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId, "oncall"] });
      setMemberForm({ user_id: "", display_name: "", position: "0" });
    },
  });

  const deleteMemberMutation = useMutation({
    mutationFn: (memberId: string) =>
      api.delete(`/rosters/${rosterId}/members/${memberId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId, "members"] });
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId, "oncall"] });
    },
  });

  const addOverrideMutation = useMutation({
    mutationFn: (data: OverrideForm) =>
      api.post(`/rosters/${rosterId}/overrides`, {
        user_id: data.user_id,
        display_name: data.display_name,
        start_time: new Date(data.start_time).toISOString(),
        end_time: new Date(data.end_time).toISOString(),
        reason: data.reason,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId, "overrides"] });
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId, "oncall"] });
      setOverrideForm({ user_id: "", display_name: "", start_time: "", end_time: "", reason: "" });
    },
  });

  const deleteOverrideMutation = useMutation({
    mutationFn: (overrideId: string) =>
      api.delete(`/rosters/${rosterId}/overrides/${overrideId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId, "overrides"] });
      queryClient.invalidateQueries({ queryKey: ["roster", rosterId, "oncall"] });
    },
  });

  if (isNew) {
    return (
      <div className="space-y-6">
        <div className="flex items-center gap-4">
          <Link to="/rosters" className="text-muted-foreground hover:text-foreground text-sm">&larr; Rosters</Link>
        </div>

        <h1 className="text-2xl font-bold">Create Roster</h1>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            createRosterMutation.mutate(rosterForm);
          }}
        >
          <Card>
            <CardHeader><CardTitle>Roster Details</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium">Name</label>
                  <Input
                    value={rosterForm.name}
                    onChange={(e) => setRosterForm({ ...rosterForm, name: e.target.value })}
                    required
                  />
                </div>
                <div>
                  <label className="text-sm font-medium">Timezone</label>
                  <Input
                    value={rosterForm.timezone}
                    onChange={(e) => setRosterForm({ ...rosterForm, timezone: e.target.value })}
                    placeholder="e.g. America/New_York"
                    required
                  />
                </div>
              </div>
              <div className="grid grid-cols-3 gap-4">
                <div>
                  <label className="text-sm font-medium">Rotation Type</label>
                  <Input
                    value={rosterForm.rotation_type}
                    onChange={(e) => setRosterForm({ ...rosterForm, rotation_type: e.target.value })}
                    placeholder="daily, weekly"
                    required
                  />
                </div>
                <div>
                  <label className="text-sm font-medium">Rotation Length (days)</label>
                  <Input
                    type="number"
                    min="1"
                    value={rosterForm.rotation_length}
                    onChange={(e) => setRosterForm({ ...rosterForm, rotation_length: e.target.value })}
                    required
                  />
                </div>
                <div>
                  <label className="text-sm font-medium">Handoff Time</label>
                  <Input
                    type="time"
                    value={rosterForm.handoff_time}
                    onChange={(e) => setRosterForm({ ...rosterForm, handoff_time: e.target.value })}
                    required
                  />
                </div>
              </div>
              <div>
                <label className="text-sm font-medium">Start Date</label>
                <Input
                  type="date"
                  value={rosterForm.start_date}
                  onChange={(e) => setRosterForm({ ...rosterForm, start_date: e.target.value })}
                  required
                />
              </div>
              <div className="flex gap-2">
                <Button type="submit" disabled={createRosterMutation.isPending}>
                  {createRosterMutation.isPending ? "Creating..." : "Create Roster"}
                </Button>
                <Link to="/rosters">
                  <Button type="button" variant="outline">Cancel</Button>
                </Link>
              </div>
              {createRosterMutation.isSuccess && (
                <p className="text-sm text-green-600">Roster created successfully.</p>
              )}
              {createRosterMutation.isError && (
                <p className="text-sm text-destructive">
                  Error: {createRosterMutation.error?.message ?? "Failed to create roster"}
                </p>
              )}
            </CardContent>
          </Card>
        </form>
      </div>
    );
  }

  if (isLoading) return <p className="text-muted-foreground">Loading...</p>;
  if (!roster) return <p className="text-muted-foreground">Roster not found</p>;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/rosters" className="text-muted-foreground hover:text-foreground text-sm">&larr; Rosters</Link>
      </div>

      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">{roster.name}</h1>
        <a
          href={`/api/v1/rosters/${rosterId}/export.ics`}
          className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
          download
        >
          <Calendar className="h-4 w-4" />
          Export iCal
        </a>
      </div>

      {onCallData?.on_call && (
        <Card>
          <CardContent className="flex items-center gap-3 py-4">
            <Badge>On Call</Badge>
            <span className="text-sm font-medium">{onCallData.on_call.user_id}</span>
            {onCallData.on_call.is_override && (
              <Badge variant="secondary">Override</Badge>
            )}
            <span className="text-sm text-muted-foreground">
              {formatRelativeTime(onCallData.on_call.shift_start)} &mdash; ends {new Date(onCallData.on_call.shift_end).toLocaleString()}
            </span>
          </CardContent>
        </Card>
      )}

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Configuration</CardTitle></CardHeader>
          <CardContent className="space-y-2 text-sm">
            <div><span className="text-muted-foreground">Timezone:</span> {roster.timezone}</div>
            <div><span className="text-muted-foreground">Rotation:</span> <span className="capitalize">{roster.rotation_type}</span> ({roster.rotation_length} days)</div>
            <div><span className="text-muted-foreground">Handoff:</span> {roster.handoff_time}</div>
            <div><span className="text-muted-foreground">Start Date:</span> {roster.start_date}</div>
            <div><span className="text-muted-foreground">Follow the Sun:</span> {roster.is_follow_the_sun ? "Yes" : "No"}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Members ({members.length})</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {members.length > 0 ? (
              <ul className="space-y-2">
                {members.map((m) => (
                  <li key={m.id} className="flex items-center gap-2 text-sm">
                    <span className="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-primary-foreground text-xs font-bold">
                      {m.position + 1}
                    </span>
                    <span className="flex-1">{m.display_name}</span>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => deleteMemberMutation.mutate(m.id)}
                      disabled={deleteMemberMutation.isPending}
                    >
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted-foreground">No members</p>
            )}

            <form
              className="space-y-2 border-t pt-4"
              onSubmit={(e) => {
                e.preventDefault();
                addMemberMutation.mutate(memberForm);
              }}
            >
              <p className="text-xs font-medium text-muted-foreground">Add Member</p>
              <div className="grid grid-cols-3 gap-2">
                <Input
                  placeholder="User ID"
                  value={memberForm.user_id}
                  onChange={(e) => setMemberForm({ ...memberForm, user_id: e.target.value })}
                  required
                />
                <Input
                  placeholder="Display Name"
                  value={memberForm.display_name}
                  onChange={(e) => setMemberForm({ ...memberForm, display_name: e.target.value })}
                  required
                />
                <Input
                  type="number"
                  placeholder="Position"
                  min="0"
                  value={memberForm.position}
                  onChange={(e) => setMemberForm({ ...memberForm, position: e.target.value })}
                  required
                />
              </div>
              <Button type="submit" size="sm" disabled={addMemberMutation.isPending}>
                {addMemberMutation.isPending ? "Adding..." : "Add"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Overrides ({overrides.length})</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {overrides.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>User</TableHead>
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
                    <TableCell className="text-sm text-muted-foreground">{new Date(o.start_time).toLocaleString()}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">{new Date(o.end_time).toLocaleString()}</TableCell>
                    <TableCell className="text-sm">{o.reason || "\u2014"}</TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => deleteOverrideMutation.mutate(o.id)}
                        disabled={deleteOverrideMutation.isPending}
                      >
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

          <form
            className="space-y-2 border-t pt-4"
            onSubmit={(e) => {
              e.preventDefault();
              addOverrideMutation.mutate(overrideForm);
            }}
          >
            <p className="text-xs font-medium text-muted-foreground">Add Override</p>
            <div className="grid grid-cols-2 gap-2">
              <Input
                placeholder="User ID"
                value={overrideForm.user_id}
                onChange={(e) => setOverrideForm({ ...overrideForm, user_id: e.target.value })}
                required
              />
              <Input
                placeholder="Display Name"
                value={overrideForm.display_name}
                onChange={(e) => setOverrideForm({ ...overrideForm, display_name: e.target.value })}
                required
              />
            </div>
            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="text-xs text-muted-foreground">Start</label>
                <Input
                  type="datetime-local"
                  value={overrideForm.start_time}
                  onChange={(e) => setOverrideForm({ ...overrideForm, start_time: e.target.value })}
                  required
                />
              </div>
              <div>
                <label className="text-xs text-muted-foreground">End</label>
                <Input
                  type="datetime-local"
                  value={overrideForm.end_time}
                  onChange={(e) => setOverrideForm({ ...overrideForm, end_time: e.target.value })}
                  required
                />
              </div>
            </div>
            <Input
              placeholder="Reason (optional)"
              value={overrideForm.reason}
              onChange={(e) => setOverrideForm({ ...overrideForm, reason: e.target.value })}
            />
            <Button type="submit" size="sm" disabled={addOverrideMutation.isPending}>
              {addOverrideMutation.isPending ? "Adding..." : "Add Override"}
            </Button>
          </form>
        </CardContent>
      </Card>

      <div className="text-xs text-muted-foreground">
        Created {formatRelativeTime(roster.created_at)} &middot; Updated {formatRelativeTime(roster.updated_at)}
      </div>
    </div>
  );
}
