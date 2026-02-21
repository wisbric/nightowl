export interface Alert {
  id: string;
  title: string;
  description: string;
  severity: "critical" | "major" | "warning" | "info";
  status: "firing" | "acknowledged" | "resolved" | "suppressed";
  source: string;
  fingerprint: string;
  cluster: string;
  namespace: string;
  service: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  occurrence_count: number;
  matched_incident_id: string | null;
  suggested_solution: string | null;
  acknowledged_by: string | null;
  acknowledged_at: string | null;
  resolved_at: string | null;
  escalation_policy_id: string | null;
  current_escalation_tier: number;
  agent_name: string | null;
  auto_resolved: boolean;
  created_at: string;
  updated_at: string;
  last_fired_at: string;
}

export interface Incident {
  id: string;
  title: string;
  severity: "critical" | "major" | "warning" | "info";
  category: string;
  status: string;
  symptoms: string;
  root_cause: string;
  solution: string;
  prevention: string;
  services: string[];
  tags: string[];
  error_patterns: string[];
  fingerprints: string[];
  environment: string;
  mttr_minutes: number | null;
  occurrence_count: number;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface Runbook {
  id: string;
  title: string;
  category: string;
  content: string;
  severity: string;
  services: string[];
  tags: string[];
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface Roster {
  id: string;
  name: string;
  timezone: string;
  rotation_type: string;
  rotation_interval_days: number;
  handoff_time: string;
  members: RosterMember[];
  created_at: string;
  updated_at: string;
}

export interface RosterMember {
  id: string;
  user_id: string;
  display_name: string;
  position: number;
}

export interface OnCallEntry {
  roster_id: string;
  roster_name: string;
  user_id: string;
  display_name: string;
  timezone: string;
  start_time: string;
  end_time: string;
  is_override: boolean;
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
  description: string;
  tiers: EscalationTier[];
  created_at: string;
  updated_at: string;
}

export interface EscalationTier {
  level: number;
  timeout_minutes: number;
  notify_type: string;
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
  user_id: string;
  user_email: string;
  action: string;
  resource_type: string;
  resource_id: string;
  diff: Record<string, unknown> | null;
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

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
}

export interface DashboardStats {
  active_alerts: number;
  open_incidents: number;
  avg_mttr_minutes: number;
  alerts_today: number;
}
