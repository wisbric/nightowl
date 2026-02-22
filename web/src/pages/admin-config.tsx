import { useState, useEffect } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import type { TenantConfigResponse } from "@/types/api";
import { TIMEZONES } from "@/lib/timezones";
import { Check } from "lucide-react";

interface ConfigForm {
  slack_workspace_url: string;
  slack_channel: string;
  twilio_sid: string;
  twilio_phone_number: string;
  default_timezone: string;
}

const emptyForm: ConfigForm = {
  slack_workspace_url: "",
  slack_channel: "",
  twilio_sid: "",
  twilio_phone_number: "",
  default_timezone: "UTC",
};

export function AdminConfigPage() {
  useTitle("Configuration");
  const queryClient = useQueryClient();

  const [form, setForm] = useState<ConfigForm>(emptyForm);
  const [saved, setSaved] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["admin-config"],
    queryFn: () => api.get<TenantConfigResponse>("/admin/config"),
  });

  useEffect(() => {
    if (data) {
      setForm({
        slack_workspace_url: data.slack_workspace_url || "",
        slack_channel: data.slack_channel || "",
        twilio_sid: data.twilio_sid || "",
        twilio_phone_number: data.twilio_phone_number || "",
        default_timezone: data.default_timezone || "UTC",
      });
    }
  }, [data]);

  const mutation = useMutation({
    mutationFn: (data: ConfigForm) =>
      api.put<TenantConfigResponse>("/admin/config", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin-config"] });
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    },
  });

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    mutation.mutate(form);
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/admin" className="text-muted-foreground hover:text-foreground text-sm">&larr; Admin</Link>
        <h1 className="text-2xl font-bold">Configuration</h1>
      </div>

      <form onSubmit={handleSubmit}>
        <div className="grid gap-6 lg:grid-cols-2">
          {/* Slack Settings */}
          <Card>
            <CardHeader>
              <CardTitle>Slack Integration</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {isLoading ? (
                <LoadingSpinner size="sm" />
              ) : (
                <>
                  <div>
                    <label className="text-sm font-medium">Workspace URL</label>
                    <Input
                      value={form.slack_workspace_url}
                      onChange={(e) => setForm({ ...form, slack_workspace_url: e.target.value })}
                      placeholder="https://your-team.slack.com"
                    />
                    <p className="text-xs text-muted-foreground mt-1">
                      Your Slack workspace URL
                    </p>
                  </div>
                  <div>
                    <label className="text-sm font-medium">Alert Channel</label>
                    <Input
                      value={form.slack_channel}
                      onChange={(e) => setForm({ ...form, slack_channel: e.target.value })}
                      placeholder="#ops-alerts"
                    />
                    <p className="text-xs text-muted-foreground mt-1">
                      Channel for alert notifications
                    </p>
                  </div>
                </>
              )}
            </CardContent>
          </Card>

          {/* Twilio Settings */}
          <Card>
            <CardHeader>
              <CardTitle>Twilio Integration</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {isLoading ? (
                <LoadingSpinner size="sm" />
              ) : (
                <>
                  <div>
                    <label className="text-sm font-medium">Account SID</label>
                    <Input
                      value={form.twilio_sid}
                      onChange={(e) => setForm({ ...form, twilio_sid: e.target.value })}
                      placeholder="ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
                    />
                    <p className="text-xs text-muted-foreground mt-1">
                      Twilio Account SID for voice/SMS escalations
                    </p>
                  </div>
                  <div>
                    <label className="text-sm font-medium">Phone Number</label>
                    <Input
                      value={form.twilio_phone_number}
                      onChange={(e) => setForm({ ...form, twilio_phone_number: e.target.value })}
                      placeholder="+1234567890"
                    />
                    <p className="text-xs text-muted-foreground mt-1">
                      Twilio phone number for outbound calls
                    </p>
                  </div>
                </>
              )}
            </CardContent>
          </Card>

          {/* General Settings */}
          <Card className="lg:col-span-2">
            <CardHeader>
              <CardTitle>General</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {isLoading ? (
                <LoadingSpinner size="sm" />
              ) : (
                <div className="max-w-sm">
                  <label className="text-sm font-medium">Default Timezone</label>
                  <Select
                    value={form.default_timezone}
                    onChange={(e) => setForm({ ...form, default_timezone: e.target.value })}
                    required
                  >
                    {TIMEZONES.map((tz) => (
                      <option key={tz} value={tz}>{tz}</option>
                    ))}
                  </Select>
                  <p className="text-xs text-muted-foreground mt-1">
                    Default timezone for schedules and reports
                  </p>
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-3 mt-6">
          <Button type="submit" disabled={mutation.isPending || isLoading}>
            {mutation.isPending ? "Saving..." : "Save Configuration"}
          </Button>
          {saved && (
            <span className="flex items-center gap-1 text-sm text-severity-ok">
              <Check className="h-4 w-4" />
              Configuration saved
            </span>
          )}
          {mutation.isError && (
            <p className="text-sm text-destructive">
              Error: {mutation.error.message}
            </p>
          )}
        </div>
      </form>
    </div>
  );
}
