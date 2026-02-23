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
import type { TenantConfigResponse, TestMessagingResponse } from "@/types/api";
import { TIMEZONES } from "@/lib/timezones";
import { Check, Wifi } from "lucide-react";

interface ConfigForm {
  messaging_provider: string;
  slack_workspace_url: string;
  slack_channel: string;
  mattermost_url: string;
  mattermost_default_channel_id: string;
  twilio_sid: string;
  twilio_phone_number: string;
  default_timezone: string;
}

const emptyForm: ConfigForm = {
  messaging_provider: "none",
  slack_workspace_url: "",
  slack_channel: "",
  mattermost_url: "",
  mattermost_default_channel_id: "",
  twilio_sid: "",
  twilio_phone_number: "",
  default_timezone: "UTC",
};

export function AdminConfigPage() {
  useTitle("Configuration");
  const queryClient = useQueryClient();

  const [form, setForm] = useState<ConfigForm>(emptyForm);
  const [saved, setSaved] = useState(false);
  const [testResult, setTestResult] = useState<TestMessagingResponse | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["admin-config"],
    queryFn: () => api.get<TenantConfigResponse>("/admin/config"),
  });

  useEffect(() => {
    if (data) {
      setForm({
        messaging_provider: data.messaging_provider || "none",
        slack_workspace_url: data.slack_workspace_url || "",
        slack_channel: data.slack_channel || "",
        mattermost_url: data.mattermost_url || "",
        mattermost_default_channel_id: data.mattermost_default_channel_id || "",
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

  const testMutation = useMutation({
    mutationFn: (req: { provider: string; bot_token?: string; url?: string }) =>
      api.post<TestMessagingResponse>("/admin/config/messaging/test", req),
    onSuccess: (data) => {
      setTestResult(data);
      setTimeout(() => setTestResult(null), 5000);
    },
  });

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    mutation.mutate(form);
  }

  function handleTestSlack() {
    setTestResult(null);
    testMutation.mutate({ provider: "slack", bot_token: form.slack_workspace_url });
  }

  function handleTestMattermost() {
    setTestResult(null);
    testMutation.mutate({ provider: "mattermost", url: form.mattermost_url });
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/admin" className="text-muted-foreground hover:text-foreground text-sm">&larr; Admin</Link>
        <h1 className="text-2xl font-bold">Configuration</h1>
      </div>

      <form onSubmit={handleSubmit}>
        <div className="grid gap-6 lg:grid-cols-2">
          {/* Messaging Provider Selection */}
          <Card className="lg:col-span-2">
            <CardHeader>
              <CardTitle>Messaging Provider</CardTitle>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <LoadingSpinner size="sm" />
              ) : (
                <div className="flex items-center gap-6">
                  {(["none", "slack", "mattermost"] as const).map((provider) => (
                    <label key={provider} className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="radio"
                        name="messaging_provider"
                        value={provider}
                        checked={form.messaging_provider === provider}
                        onChange={() => setForm({ ...form, messaging_provider: provider })}
                        className="accent-accent"
                      />
                      <span className="text-sm capitalize">{provider === "none" ? "None" : provider}</span>
                    </label>
                  ))}
                  <p className="text-xs text-muted-foreground ml-4">
                    Select the messaging platform for alert notifications and slash commands.
                  </p>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Slack Settings */}
          <Card className={form.messaging_provider !== "slack" ? "opacity-60" : ""}>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>Slack Integration</CardTitle>
                {form.messaging_provider === "slack" && (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleTestSlack}
                    disabled={testMutation.isPending}
                  >
                    <Wifi className="h-3 w-3 mr-1" />
                    Test
                  </Button>
                )}
              </div>
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
                      disabled={form.messaging_provider !== "slack"}
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
                      disabled={form.messaging_provider !== "slack"}
                    />
                    <p className="text-xs text-muted-foreground mt-1">
                      Channel for alert notifications
                    </p>
                  </div>
                  {testResult && testMutation.variables?.provider === "slack" && (
                    <div className={`text-xs p-2 rounded ${testResult.ok ? "bg-green-600/10 text-green-400" : "bg-destructive/10 text-destructive"}`}>
                      {testResult.ok
                        ? `Connected: ${testResult.bot_name} (${testResult.workspace})`
                        : `Error: ${testResult.error}`}
                    </div>
                  )}
                </>
              )}
            </CardContent>
          </Card>

          {/* Mattermost Settings */}
          <Card className={form.messaging_provider !== "mattermost" ? "opacity-60" : ""}>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>Mattermost Integration</CardTitle>
                {form.messaging_provider === "mattermost" && (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleTestMattermost}
                    disabled={testMutation.isPending}
                  >
                    <Wifi className="h-3 w-3 mr-1" />
                    Test
                  </Button>
                )}
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              {isLoading ? (
                <LoadingSpinner size="sm" />
              ) : (
                <>
                  <div>
                    <label className="text-sm font-medium">Server URL</label>
                    <Input
                      value={form.mattermost_url}
                      onChange={(e) => setForm({ ...form, mattermost_url: e.target.value })}
                      placeholder="https://mattermost.example.com"
                      disabled={form.messaging_provider !== "mattermost"}
                    />
                    <p className="text-xs text-muted-foreground mt-1">
                      Mattermost server URL
                    </p>
                  </div>
                  <div>
                    <label className="text-sm font-medium">Default Channel ID</label>
                    <Input
                      value={form.mattermost_default_channel_id}
                      onChange={(e) => setForm({ ...form, mattermost_default_channel_id: e.target.value })}
                      placeholder="abc123..."
                      disabled={form.messaging_provider !== "mattermost"}
                    />
                    <p className="text-xs text-muted-foreground mt-1">
                      Channel ID for alert notifications
                    </p>
                  </div>
                  {testResult && testMutation.variables?.provider === "mattermost" && (
                    <div className={`text-xs p-2 rounded ${testResult.ok ? "bg-green-600/10 text-green-400" : "bg-destructive/10 text-destructive"}`}>
                      {testResult.ok
                        ? `Connected: ${testResult.bot_name}`
                        : `Error: ${testResult.error}`}
                    </div>
                  )}
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
          <Card>
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
