import { useState, useEffect } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { TIMEZONES } from "@/lib/timezones";
import type { UserPreferences } from "@/types/api";
import { Check } from "lucide-react";

const TIME_RANGES = [
  { value: "1h", label: "Last 1 hour" },
  { value: "6h", label: "Last 6 hours" },
  { value: "24h", label: "Last 24 hours" },
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
];

export function SettingsPreferencesPage() {
  useTitle("Preferences");
  const queryClient = useQueryClient();
  const [saved, setSaved] = useState(false);

  const { data: prefs, isLoading } = useQuery({
    queryKey: ["user-preferences"],
    queryFn: () => api.get<UserPreferences>("/user/preferences"),
  });

  const [timezone, setTimezone] = useState("");
  const [theme, setTheme] = useState("system");
  const [notifCritical, setNotifCritical] = useState(true);
  const [notifMajor, setNotifMajor] = useState(true);
  const [notifWarning, setNotifWarning] = useState(false);
  const [notifInfo, setNotifInfo] = useState(false);
  const [timeRange, setTimeRange] = useState("24h");

  useEffect(() => {
    if (prefs) {
      setTimezone(prefs.timezone || "");
      setTheme(prefs.theme || "system");
      setNotifCritical(prefs.notifications?.critical ?? true);
      setNotifMajor(prefs.notifications?.major ?? true);
      setNotifWarning(prefs.notifications?.warning ?? false);
      setNotifInfo(prefs.notifications?.info ?? false);
      setTimeRange(prefs.dashboard?.default_time_range || "24h");
    }
  }, [prefs]);

  const mutation = useMutation({
    mutationFn: (data: UserPreferences) =>
      api.put<UserPreferences>("/user/preferences", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["user-preferences"] });
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    },
  });

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    mutation.mutate({
      timezone,
      theme,
      notifications: {
        critical: notifCritical,
        major: notifMajor,
        warning: notifWarning,
        info: notifInfo,
      },
      dashboard: {
        default_time_range: timeRange,
      },
    });
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/settings" className="text-muted-foreground hover:text-foreground text-sm">&larr; Settings</Link>
        <h1 className="text-2xl font-bold">Preferences</h1>
      </div>

      {isLoading ? (
        <LoadingSpinner />
      ) : (
        <form onSubmit={handleSubmit} className="space-y-6">
          <Card>
            <CardHeader><CardTitle>General</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              <div className="max-w-sm">
                <label className="text-sm font-medium">Timezone</label>
                <Select value={timezone} onChange={(e) => setTimezone(e.target.value)}>
                  <option value="">Use tenant default</option>
                  {TIMEZONES.map((tz) => (
                    <option key={tz} value={tz}>{tz}</option>
                  ))}
                </Select>
                <p className="text-xs text-muted-foreground mt-1">
                  Override the tenant default timezone for your account.
                </p>
              </div>

              <div>
                <label className="text-sm font-medium">Theme</label>
                <div className="mt-2 flex items-center gap-6">
                  {(["system", "dark", "light"] as const).map((t) => (
                    <label key={t} className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="radio"
                        name="theme"
                        value={t}
                        checked={theme === t}
                        onChange={() => setTheme(t)}
                        className="accent-accent"
                      />
                      <span className="text-sm capitalize">{t}</span>
                    </label>
                  ))}
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle>Notifications</CardTitle></CardHeader>
            <CardContent className="space-y-3">
              <p className="text-xs text-muted-foreground">
                Choose which severity levels trigger notifications.
              </p>
              {[
                { label: "Critical", checked: notifCritical, set: setNotifCritical },
                { label: "Major", checked: notifMajor, set: setNotifMajor },
                { label: "Warning", checked: notifWarning, set: setNotifWarning },
                { label: "Info", checked: notifInfo, set: setNotifInfo },
              ].map(({ label, checked, set }) => (
                <label key={label} className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={(e) => set(e.target.checked)}
                    className="h-4 w-4 rounded border-input accent-accent"
                  />
                  <span className="text-sm">{label}</span>
                </label>
              ))}
            </CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle>Dashboard</CardTitle></CardHeader>
            <CardContent>
              <div className="max-w-sm">
                <label className="text-sm font-medium">Default Time Range</label>
                <Select value={timeRange} onChange={(e) => setTimeRange(e.target.value)}>
                  {TIME_RANGES.map((tr) => (
                    <option key={tr.value} value={tr.value}>{tr.label}</option>
                  ))}
                </Select>
                <p className="text-xs text-muted-foreground mt-1">
                  Default time range shown on the dashboard.
                </p>
              </div>
            </CardContent>
          </Card>

          <div className="flex items-center gap-3">
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? "Saving..." : "Save Preferences"}
            </Button>
            {saved && (
              <span className="flex items-center gap-1 text-sm text-severity-ok">
                <Check className="h-4 w-4" />
                Preferences saved
              </span>
            )}
            {mutation.isError && (
              <p className="text-sm text-destructive">
                Error: {mutation.error.message}
              </p>
            )}
          </div>
        </form>
      )}
    </div>
  );
}
