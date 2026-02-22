/**
 * Shared IANA timezone list used across user management, roster creation,
 * and tenant configuration. All timezone dropdowns must use this list to
 * ensure consistent matching when filtering users by roster timezone.
 */
export const TIMEZONES = [
  "UTC",
  "Pacific/Auckland",
  "Australia/Sydney",
  "Asia/Tokyo",
  "Asia/Shanghai",
  "Asia/Kolkata",
  "Asia/Dubai",
  "Europe/Moscow",
  "Europe/Istanbul",
  "Europe/Berlin",
  "Europe/Paris",
  "Europe/London",
  "America/Sao_Paulo",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/Anchorage",
  "Pacific/Honolulu",
] as const;

export type Timezone = (typeof TIMEZONES)[number];
