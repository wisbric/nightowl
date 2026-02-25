// Response wrappers matching actual API responses
export interface AlertsResponse {
  alerts: Alert[];
  count: number;
}

export interface IncidentsResponse {
  items: Incident[];
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export interface SearchResponse {
  results: SearchResult[];
  count: number;
  query: string;
}

export interface SearchResult {
  id: string;
  title: string;
  severity: string;
  category: string;
  services: string[];
  tags: string[];
  solution: string;
  symptoms: string;
  root_cause: string;
  runbook_id?: string | null;
  rank: number;
  title_highlight: string;
  symptoms_highlight: string;
  solution_highlight: string;
  resolution_count: number;
  created_at: string;
}

export interface RostersResponse {
  rosters: Roster[];
  count: number;
}

export interface PoliciesResponse {
  policies: EscalationPolicy[];
  count: number;
}

export interface Alert {
  id: string;
  title: string;
  description?: string;
  severity: string;
  status: string;
  source: string;
  fingerprint: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  occurrence_count: number;
  matched_incident_id?: string;
  suggested_solution?: string;
  runbook_url?: string;
  first_fired_at: string;
  last_fired_at: string;
  created_at: string;
}

export interface Incident {
  id: string;
  title: string;
  severity: string;
  category: string;
  symptoms: string | null;
  root_cause: string | null;
  solution: string | null;
  prevention: string | null;
  services: string[];
  tags: string[];
  error_patterns: string[];
  fingerprints: string[];
  clusters: string[];
  namespaces: string[];
  environment: string;
  runbook_id?: string | null;
  runbook_title?: string | null;
  runbook_content?: string | null;
  resolution_count: number;
  created_at: string;
  updated_at: string;
}

// --- Roster v2 types ---

export interface Roster {
  id: string;
  name: string;
  description?: string;
  timezone: string;
  handoff_time: string;
  handoff_day: number; // 0=Sun..6=Sat
  schedule_weeks_ahead: number;
  max_consecutive_weeks: number;
  is_follow_the_sun: boolean;
  linked_roster_id?: string;
  active_hours_start?: string;
  active_hours_end?: string;
  escalation_policy_id?: string;
  end_date?: string | null;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface RosterMember {
  id: string;
  roster_id: string;
  user_id: string;
  display_name: string;
  is_active: boolean;
  joined_at: string;
  left_at?: string;
  primary_weeks_served: number;
  secondary_weeks_served: number;
}

// --- Coverage types ---

export interface CoverageResponse {
  from: string;
  to: string;
  resolution_minutes: number;
  rosters: CoverageRoster[];
  slots: CoverageSlot[];
  gap_summary: GapSummary;
}

export interface CoverageRoster {
  id: string;
  name: string;
  timezone: string;
  active_hours_start?: string;
  active_hours_end?: string;
  is_follow_the_sun: boolean;
}

export interface CoverageSlot {
  time: string;
  coverage: CoverageSlotRoster[];
  gap: boolean;
}

export interface CoverageSlotRoster {
  roster_id: string;
  roster_name: string;
  primary: string;
  secondary?: string;
  source: string;
}

export interface GapSummary {
  total_gap_hours: number;
  gaps: GapInfo[];
}

export interface GapInfo {
  start: string;
  end: string;
  duration_hours: number;
}

export interface MembersResponse {
  members: RosterMember[];
  count: number;
}

export interface ScheduleEntry {
  id: string;
  roster_id: string;
  week_start: string;
  week_end: string;
  primary_user_id?: string | null;
  primary_display_name?: string;
  secondary_user_id?: string | null;
  secondary_display_name?: string;
  is_locked: boolean;
  generated: boolean;
  notes?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ScheduleResponse {
  schedule: ScheduleEntry[];
  count: number;
}

export interface OnCallResponse {
  roster_id: string;
  roster_name: string;
  queried_at: string;
  source: "override" | "schedule" | "unassigned";
  primary: OnCallEntry | null;
  secondary: OnCallEntry | null;
  week_start?: string;
  active_override?: Override | null;
}

export interface OnCallEntry {
  user_id: string;
  display_name: string;
}

export interface Override {
  id: string;
  roster_id: string;
  user_id: string;
  display_name: string;
  start_at: string;
  end_at: string;
  reason?: string;
  created_by?: string;
  created_at: string;
}

export interface OverridesResponse {
  overrides: Override[];
  count: number;
}

// --- Escalation types ---

export interface EscalationPolicy {
  id: string;
  name: string;
  description?: string;
  tiers: EscalationTier[];
  repeat_count?: number;
  created_at: string;
  updated_at: string;
}

export interface EscalationTier {
  tier: number;
  timeout_minutes: number;
  notify_via: string[];
  targets: string[];
}

export interface EscalationEvent {
  id: string;
  alert_id: string;
  policy_id: string;
  tier: number;
  notified_at: string;
  notify_type: string;
  target: string;
}

// --- Audit ---

export interface AuditEntry {
  id: string;
  user_id: string | null;
  api_key_id: string | null;
  action: string;
  resource: string;
  resource_id: string;
  detail: string;
  ip_address: string;
  user_agent: string;
  created_at: string;
}

// --- Runbooks ---

export interface Runbook {
  id: string;
  title: string;
  content: string;
  category: string;
  is_template: boolean;
  tags: string[];
  created_at: string;
  updated_at: string;
}

export interface RunbooksResponse {
  items: Runbook[];
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export interface IncidentHistoryEntry {
  id: string;
  incident_id: string;
  field: string;
  old_value: string;
  new_value: string;
  changed_by: string;
  created_at: string;
}

export interface DryRunResponse {
  policy_id: string;
  policy_name: string;
  steps: DryRunStep[];
  total_time_minutes: number;
}

export interface DryRunStep {
  tier: number;
  timeout_minutes: number;
  cumulative_minutes: number;
  notify_via: string[];
  targets: string[];
  action: string;
}

// --- Users ---

export interface User {
  id: string;
  email: string;
  display_name: string;
  role: string;
  tenant_slug: string;
  created_at: string;
}

export interface UsersResponse {
  users: UserDetail[];
  count: number;
}

export interface UserDetail {
  id: string;
  email: string;
  display_name: string;
  role: string;
  timezone: string;
  phone?: string;
  slack_user_id?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

// --- API Keys ---

export interface ApiKeysResponse {
  keys: ApiKeyDetail[];
  count: number;
}

export interface ApiKeyDetail {
  id: string;
  key_prefix: string;
  description: string;
  role: string;
  scopes: string[];
  last_used?: string;
  expires_at?: string;
  created_at: string;
}

export interface ApiKeyCreateResponse extends ApiKeyDetail {
  raw_key: string;
}

// --- Personal Access Tokens ---

export interface PersonalAccessToken {
  id: string;
  user_id: string;
  name: string;
  prefix: string;
  expires_at: string | null;
  last_used_at: string | null;
  created_at: string;
}

export interface PATCreateResponse {
  id: string;
  user_id: string;
  name: string;
  prefix: string;
  expires_at: string | null;
  last_used_at: string | null;
  created_at: string;
  raw_token: string;
}

export interface PATListResponse {
  tokens: PersonalAccessToken[];
  count: number;
}

// --- Admin ---

export interface TenantConfigResponse {
  messaging_provider: string;
  slack_workspace_url: string;
  slack_channel: string;
  mattermost_url: string;
  mattermost_default_channel_id: string;
  twilio_sid: string;
  twilio_phone_number: string;
  default_timezone: string;
  bookowl_api_url: string;
  bookowl_api_key: string;
  updated_at: string;
}

export interface TestMessagingResponse {
  ok: boolean;
  error?: string;
  bot_name?: string;
  workspace?: string;
}

export interface StatusResponse {
  status: string;
  version: string;
  commit_sha: string;
  uptime: string;
  uptime_seconds: number;
  database: string;
  database_latency_ms: number;
  redis: string;
  redis_latency_ms: number;
  last_alert_at: string | null;
}

// --- BookOwl Integration ---

export interface BookOwlStatusResponse {
  integrated: boolean;
  url?: string;
}

export interface BookOwlRunbookListItem {
  id: string;
  title: string;
  slug: string;
  tags: string[];
  url: string;
  updated_at: string;
}

export interface BookOwlRunbookListResponse {
  items: BookOwlRunbookListItem[];
  total: number;
  limit: number;
  offset: number;
}

export interface BookOwlRunbookDetail {
  id: string;
  title: string;
  slug: string;
  content_text: string;
  content_html: string;
  url: string;
  tags: string[];
  updated_at: string;
}

export interface BookOwlPostMortemResponse {
  id: string;
  url: string;
  title: string;
}

export interface TestBookOwlResponse {
  ok: boolean;
  error?: string;
  count?: number;
}

// --- User Preferences ---

export interface UserPreferences {
  timezone?: string;
  theme?: string;
  notifications?: {
    critical: boolean;
    major: boolean;
    warning: boolean;
    info: boolean;
  };
  dashboard?: {
    default_time_range?: string;
  };
}
