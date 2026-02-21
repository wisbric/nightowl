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
  resolution_count: number;
  created_at: string;
  updated_at: string;
}

export interface Roster {
  id: string;
  name: string;
  timezone: string;
  rotation_type: string;
  rotation_length: number;
  handoff_time: string;
  is_follow_the_sun: boolean;
  start_date: string;
  members?: RosterMember[];
  created_at: string;
  updated_at: string;
}

export interface RosterMember {
  id: string;
  user_id: string;
  display_name: string;
  position: number;
}

export interface OnCallResponse {
  on_call: OnCallEntry | null;
  message?: string;
}

export interface OnCallEntry {
  user_id: string;
  roster_id: string;
  roster_name: string;
  is_override: boolean;
  shift_start: string;
  shift_end: string;
}

export interface Override {
  id: string;
  roster_id: string;
  user_id: string;
  display_name: string;
  start_time: string;
  end_time: string;
  reason: string;
  created_at: string;
}

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

export interface User {
  id: string;
  email: string;
  display_name: string;
  role: string;
  tenant_slug: string;
  created_at: string;
}
