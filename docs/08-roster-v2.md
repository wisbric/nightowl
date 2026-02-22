# NightOwl — Roster v2: Explicit Schedule Management

> This spec replaces the calculated rotation model (Phase 4.1/4.2) with an explicit, editable weekly schedule. The member pool, overrides, follow-the-sun, escalation, and calendar export remain.

---

## 1. Problem with Current Model

The current roster calculates who's on-call using:
```
position = (days_since_start / rotation_length) % member_count
```

This is rigid and doesn't support real-world needs:
- No forward visibility — you can't see who's on-call 3 weeks from now
- No way to edit a specific future week
- No way to handle leave or planned absences beyond overrides
- Primary/secondary assignment is implicit (position 0 = primary, position 1 = secondary)
- No fair distribution tracking

**New model:** Explicit `roster_schedule` rows — one per week — with named primary and secondary. Auto-generated from the member pool, but fully editable.

---

## 2. Data Model Changes

### 2.1 Keep As-Is

These tables remain unchanged:

- **`rosters`** — roster definition (name, timezone, handoff_time, follow-the-sun config, escalation_policy_id)
- **`roster_members`** — pool of available people (drop `position` column, add `is_active`)
- **`roster_overrides`** — short-notice day-level coverage swaps (sick days, emergencies)
- **`escalation_policies`** — unchanged
- **`escalation_events`** — unchanged

### 2.2 Modify `rosters` Table

```sql
ALTER TABLE rosters
    DROP COLUMN rotation_type,       -- no longer needed
    DROP COLUMN rotation_length,     -- no longer needed
    DROP COLUMN start_date,          -- schedule has its own dates
    ADD COLUMN schedule_weeks_ahead INTEGER NOT NULL DEFAULT 12,  -- how many weeks to auto-generate
    ADD COLUMN max_consecutive_weeks INTEGER NOT NULL DEFAULT 2;  -- max weeks same person is primary in a row
```

Updated schema:

```sql
CREATE TABLE rosters (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                  TEXT NOT NULL,
    description           TEXT,
    timezone              TEXT NOT NULL,
    handoff_time          TIME NOT NULL DEFAULT '09:00',
    handoff_day           INTEGER NOT NULL DEFAULT 1,  -- 0=Sunday, 1=Monday, ..., 6=Saturday

    -- Schedule generation
    schedule_weeks_ahead  INTEGER NOT NULL DEFAULT 12,
    max_consecutive_weeks INTEGER NOT NULL DEFAULT 2,

    -- Follow-the-sun
    is_follow_the_sun     BOOLEAN DEFAULT false,
    linked_roster_id      UUID REFERENCES rosters(id),
    active_hours_start    TIME,          -- e.g., 08:00 local time
    active_hours_end      TIME,          -- e.g., 20:00 local time

    -- Escalation
    escalation_policy_id  UUID REFERENCES escalation_policies(id),

    -- Status
    is_active             BOOLEAN NOT NULL DEFAULT true,
    end_date              DATE,

    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 2.3 Modify `roster_members` Table

```sql
CREATE TABLE roster_members (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    roster_id   UUID NOT NULL REFERENCES rosters(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id),
    is_active   BOOLEAN NOT NULL DEFAULT true,  -- can be scheduled (set false for leave/departure)
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    left_at     TIMESTAMPTZ,

    UNIQUE(roster_id, user_id)
);

CREATE INDEX idx_roster_members_roster ON roster_members(roster_id);
CREATE INDEX idx_roster_members_active ON roster_members(roster_id) WHERE is_active = true;
```

**Key change:** removed `position` column. Order is no longer used for rotation calculation — the schedule is explicit.

### 2.4 New: `roster_schedule` Table

```sql
CREATE TABLE roster_schedule (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    roster_id       UUID NOT NULL REFERENCES rosters(id) ON DELETE CASCADE,
    week_start      DATE NOT NULL,           -- Monday of the week (or handoff_day)
    week_end        DATE NOT NULL,           -- Sunday of the week (computed)
    primary_user_id UUID REFERENCES users(id),
    secondary_user_id UUID REFERENCES users(id),
    is_locked       BOOLEAN NOT NULL DEFAULT false,  -- locked = don't overwrite on re-generate
    generated       BOOLEAN NOT NULL DEFAULT true,   -- true = auto-generated, false = manually set
    notes           TEXT,                    -- e.g., "Stefan on leave, Max covering"
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(roster_id, week_start),
    CHECK (primary_user_id IS DISTINCT FROM secondary_user_id),
    CHECK (week_end > week_start)
);

CREATE INDEX idx_roster_schedule_roster ON roster_schedule(roster_id, week_start);
CREATE INDEX idx_roster_schedule_current ON roster_schedule(roster_id, week_start, week_end);
```

**Key design decisions:**
- One row per roster per week
- `primary_user_id` and `secondary_user_id` are nullable — allows empty weeks (nobody assigned yet)
- `is_locked` prevents auto-regeneration from overwriting manual edits
- `generated` tracks whether a human or the algorithm set this row
- `notes` allows context ("Anna on parental leave", "Swapped with Max")
- `week_start` aligned to `handoff_day` from the roster config

### 2.5 Overrides Still Apply

`roster_overrides` remain for **intra-week** changes:
- Someone gets sick on Wednesday — override covers Wed–Fri
- Override takes precedence over the schedule for the covered time window
- Override UI and logic unchanged

**Resolution order for "who's on call right now?":**
1. Check `roster_overrides` for active override at current time → use that person
2. Check `roster_schedule` for current week → use primary/secondary
3. If no schedule entry exists → return "unassigned" (not an error, but triggers a warning)

---

## 3. Schedule Generation Algorithm

### 3.1 Trigger

Schedule generation runs:
- When a roster is created
- When members are added/removed
- When an admin clicks "Regenerate Schedule"
- Weekly via worker cron: top up schedule to maintain `schedule_weeks_ahead` weeks of future coverage

### 3.2 Algorithm

```
INPUT:
  - active_members: list of users with is_active=true
  - existing_schedule: locked/manual entries that must not be overwritten
  - weeks_to_generate: from next unscheduled week to schedule_weeks_ahead
  - max_consecutive: max times same person can be primary in a row
  - history: last N weeks of actual schedule (for fairness tracking)

PROCESS:
  1. Count total_primary_weeks per member (from history + existing locked entries)
  2. For each week needing assignment:
     a. Skip if is_locked = true (manual edit, don't touch)
     b. Sort eligible members by total_primary_weeks ASC (least served first)
     c. Filter out anyone who has been primary for max_consecutive weeks in a row
     d. Assign primary = first eligible member
     e. Assign secondary = next eligible member (different from primary)
     f. Increment total_primary_weeks for the assigned primary
     g. Insert/update roster_schedule row with generated=true

OUTPUT:
  - roster_schedule rows for all weeks in the range
  - Locked rows untouched
  - Fair distribution: primary duty spread evenly across all active members
```

### 3.3 Fairness

The algorithm tracks "primary weeks served" and always picks the person who has served the fewest. Over time this naturally balances the load.

Example with 4 members over 8 weeks:
```
Week 1: Primary=Stefan  Secondary=Max      (Stefan: 1, Max: 0, Anna: 0, Lars: 0)
Week 2: Primary=Anna    Secondary=Lars     (Stefan: 1, Max: 0, Anna: 1, Lars: 0)
Week 3: Primary=Max     Secondary=Lars     (Stefan: 1, Max: 1, Anna: 1, Lars: 0)
Week 4: Primary=Lars    Secondary=Stefan   (Stefan: 1, Max: 1, Anna: 1, Lars: 1)
Week 5: Primary=Stefan  Secondary=Anna     (Stefan: 2, Max: 1, Anna: 1, Lars: 1)
Week 6: Primary=Max     Secondary=Anna     (Stefan: 2, Max: 2, Anna: 1, Lars: 1)
Week 7: Primary=Anna    Secondary=Lars     (Stefan: 2, Max: 2, Anna: 2, Lars: 1)
Week 8: Primary=Lars    Secondary=Stefan   (Stefan: 2, Max: 2, Anna: 2, Lars: 2)
```

### 3.4 Edge Cases

- **< 2 active members:** Cannot assign both primary and secondary. Assign primary only, secondary=NULL. Log a warning.
- **1 active member:** That person is always primary. No secondary. Dashboard shows a warning.
- **0 active members:** No schedule generated. Dashboard shows critical warning.
- **Member deactivated mid-schedule:** Re-generate future unlocked weeks. Past weeks untouched.
- **Member added:** Re-generate future unlocked weeks to include them.

---

## 4. API Changes

### 4.1 Schedule Endpoints (New)

```
GET    /api/v1/rosters/:id/schedule
       ?from=2026-02-24&to=2026-05-18
       → Returns list of roster_schedule entries for date range
       → Default: current week to schedule_weeks_ahead weeks out
       → Include user display names, override info

GET    /api/v1/rosters/:id/schedule/:weekStart
       → Single week detail (schedule + any overrides that intersect)

PUT    /api/v1/rosters/:id/schedule/:weekStart
       Body: { "primary_user_id": "uuid", "secondary_user_id": "uuid", "notes": "..." }
       → Manual assignment for a specific week
       → Sets is_locked=true, generated=false
       → Validates both users are roster members

DELETE /api/v1/rosters/:id/schedule/:weekStart/lock
       → Unlocks a week (sets is_locked=false)
       → Next regeneration may overwrite it

POST   /api/v1/rosters/:id/schedule/generate
       Body: { "from": "2026-03-01", "weeks": 12 }
       → Force regeneration of unlocked weeks
       → Returns the generated schedule
```

### 4.2 Modified On-Call Endpoint

```
GET    /api/v1/rosters/:id/oncall
       ?at=<RFC3339>    (optional, defaults to now)
       → Resolution order: override → schedule → unassigned
       → Response:
       {
         "roster_id": "uuid",
         "roster_name": "DE On-Call",
         "queried_at": "2026-02-23T14:30:00+01:00",
         "source": "schedule",          // "override" | "schedule" | "unassigned"
         "primary": {
           "user_id": "uuid",
           "display_name": "Stefan K.",
           "email": "stefan@example.com"
         },
         "secondary": {
           "user_id": "uuid",
           "display_name": "Max M.",
           "email": "max@example.com"
         },
         "week_start": "2026-02-24",
         "active_override": null        // or override details if applicable
       }
```

### 4.3 Modified Member Endpoints

```
POST   /api/v1/rosters/:id/members
       Body: { "user_id": "uuid" }
       → Add member to pool (is_active=true)
       → Trigger schedule regeneration for future unlocked weeks

DELETE /api/v1/rosters/:id/members/:userId
       → Soft deactivate: set is_active=false, left_at=now()
       → Trigger schedule regeneration for future unlocked weeks
       → Don't delete — keep for history

PUT    /api/v1/rosters/:id/members/:userId
       Body: { "is_active": true/false }
       → Toggle active status (e.g., long leave and return)
       → Trigger schedule regeneration

GET    /api/v1/rosters/:id/members
       → List all members with is_active status, joined_at, primary_weeks_served count
```

### 4.4 Override Endpoints (Unchanged)

```
POST   /api/v1/rosters/:id/overrides
GET    /api/v1/rosters/:id/overrides
DELETE /api/v1/rosters/:id/overrides/:id
```

### 4.5 Calendar Export (Updated)

```
GET    /api/v1/rosters/:id/export.ics
       → Generate from roster_schedule (not calculated rotation)
       → Include: primary/secondary per week, overrides, handoff times
       → Future weeks from schedule, indicate unassigned weeks
```

---

## 5. Frontend Changes

### 5.1 Roster Detail Page — New Layout

```
┌──────────────────────────────────────────────────────────────┐
│  ← Back to Rosters                                           │
│                                                              │
│  DE On-Call                                     [Edit] [⚙️]  │
│  Europe/Berlin · Handoff: Monday 09:00                       │
│  Escalation: Standard 3-tier                                 │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  ON-CALL NOW                                                 │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  🟢 Primary: Stefan K.    📞 +49 170 ...             │  │
│  │  🔵 Secondary: Max M.     📞 +49 151 ...             │  │
│  │  Source: Schedule (Week of Feb 24)                     │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  SCHEDULE                    [Regenerate] [+ Add Week]       │
│                                                              │
│  ┌──────────┬───────────────┬───────────────┬──────┬──────┐  │
│  │ Week of  │ Primary       │ Secondary     │ Lock │ Edit │  │
│  ├──────────┼───────────────┼───────────────┼──────┼──────┤  │
│  │ Feb 17 ● │ Max M.        │ Anna S.       │  🔒  │      │  │  ← past (dimmed)
│  │ Feb 24 ◉ │ Stefan K.     │ Max M.        │  🔒  │  ✏️  │  ← current week
│  │ Mar 03   │ Anna S.       │ Lars B.       │      │  ✏️  │  ← future
│  │ Mar 10   │ Lars B.       │ Stefan K.     │      │  ✏️  │
│  │ Mar 17   │ Stefan K.     │ Anna S.       │  🔒  │  ✏️  │  ← manually set
│  │ Mar 24   │ Max M.        │ Lars B.       │      │  ✏️  │
│  │ Mar 31   │ Anna S.       │ Stefan K.     │      │  ✏️  │
│  │ Apr 07   │ Lars B.       │ Max M.        │      │  ✏️  │
│  │ ...      │               │               │      │      │
│  └──────────┴───────────────┴───────────────┴──────┴──────┘  │
│                                                              │
│  ● Past  ◉ Current  🔒 Locked (won't auto-regenerate)       │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  MEMBERS (4 active)                          [+ Add Member]  │
│  ┌───────────────┬────────────┬──────────────┬────────────┐  │
│  │ Name          │ Status     │ Primary Weeks│ Actions    │  │
│  ├───────────────┼────────────┼──────────────┼────────────┤  │
│  │ Stefan K.     │ 🟢 Active  │ 5            │ Deactivate │  │
│  │ Max M.        │ 🟢 Active  │ 4            │ Deactivate │  │
│  │ Anna S.       │ 🟢 Active  │ 4            │ Deactivate │  │
│  │ Lars B.       │ 🟢 Active  │ 3            │ Deactivate │  │
│  │ Tobias W.     │ ⚪ Inactive│ 2            │ Activate   │  │
│  └───────────────┴────────────┴──────────────┴────────────┘  │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  OVERRIDES                                   [+ Add Override]│
│  ┌───────────────┬─────────────────┬──────────┬────────────┐ │
│  │ Covering      │ Period          │ Reason   │ Actions    │ │
│  ├───────────────┼─────────────────┼──────────┼────────────┤ │
│  │ Max M.        │ Feb 26–Feb 28   │ Stefan   │ Remove     │ │
│  │               │                 │ sick     │            │ │
│  └───────────────┴─────────────────┴──────────┴────────────┘ │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  📅 iCal Export    📊 Fairness Report    📜 Shift History   │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 5.2 Edit Week Dialog

When clicking ✏️ on a schedule row:

```
┌─────────────────────────────────────────┐
│  Edit Schedule: Week of Mar 03          │
│                                         │
│  Primary:    [ Anna S.        ▾ ]       │
│  Secondary:  [ Lars B.        ▾ ]       │
│                                         │
│  Notes:      [ _________________ ]      │
│                                         │
│  ☑ Lock this week                       │
│    (prevents auto-regeneration)         │
│                                         │
│              [ Cancel ]  [ Save ]       │
└─────────────────────────────────────────┘
```

- Dropdown shows only active roster members
- Cannot select same person for primary and secondary
- Saving sets `is_locked=true` and `generated=false`
- Notes optional — useful for context ("Swapped with Max, Anna on holiday")

### 5.3 Regenerate Confirmation

When clicking "Regenerate":

```
┌─────────────────────────────────────────┐
│  Regenerate Schedule                    │
│                                         │
│  This will reassign primary/secondary   │
│  for all UNLOCKED future weeks based    │
│  on fair rotation.                      │
│                                         │
│  🔒 4 locked weeks will not be changed  │
│  📝 8 weeks will be regenerated         │
│                                         │
│  Generate [ 12 ] weeks from today       │
│                                         │
│              [ Cancel ]  [ Regenerate ] │
└─────────────────────────────────────────┘
```

### 5.4 Dashboard On-Call Widget

Updated to show schedule context:

```
┌─── On-Call Now ──────────────────────────────┐
│                                              │
│  🇩🇪 DE On-Call (Week of Feb 24)              │
│     Primary: Stefan K.                       │
│     Secondary: Max M.                        │
│                                              │
│  🇳🇿 NZ On-Call (Week of Feb 24)              │
│     Primary: Jamie R.                        │
│     Secondary: Chris T.                      │
│                                              │
│  Next handoff: Mon Mar 03 09:00 CET          │
└──────────────────────────────────────────────┘
```

### 5.5 Fairness Report

Accessible from roster detail page. Shows distribution stats:

```
┌─── Fairness Report: DE On-Call ──────────────┐
│  Last 12 weeks                               │
│                                              │
│  ┌───────────┬─────────┬───────────┬───────┐ │
│  │ Member    │ Primary │ Secondary │ Total │ │
│  ├───────────┼─────────┼───────────┼───────┤ │
│  │ Stefan K. │ 3       │ 3         │ 6     │ │
│  │ Max M.    │ 3       │ 3         │ 6     │ │
│  │ Anna S.   │ 3       │ 3         │ 6     │ │
│  │ Lars B.   │ 3       │ 3         │ 6     │ │
│  └───────────┴─────────┴───────────┴───────┘ │
│                                              │
│  Distribution: ████████████████████ Even ✓   │
└──────────────────────────────────────────────┘
```

---

## 6. Worker Changes

### 6.1 Schedule Top-Up (Cron)

The worker runs a weekly job (e.g., every Monday at 00:00 UTC):

1. For each active roster:
   - Count scheduled weeks from today
   - If fewer than `schedule_weeks_ahead`, generate more
   - Only fill unlocked, unscheduled weeks
   - Log: "Roster DE-oncall: generated 4 new weeks (Mar 31 – Apr 21)"

### 6.2 On-Call Resolution (Updated)

The escalation engine's "who's on call" lookup changes from:
```
old: calculate position from start_date + rotation_length
new: SELECT primary_user_id, secondary_user_id FROM roster_schedule
     WHERE roster_id = $1 AND week_start <= $2 AND week_end >= $2
     then check roster_overrides for the specific timestamp
```

### 6.3 Handoff Notifications (Updated)

At handoff time (e.g., Monday 09:00 CET):
1. Look up this week's schedule entry
2. Look up last week's schedule entry
3. If primary changed → send handoff notification:
   - Outgoing: "Your on-call shift has ended. Open alerts: 3"
   - Incoming: "You are now primary on-call. Open alerts: 3. Here's what happened last week: ..."
   - Slack channel: "🔄 On-call handoff: Stefan K. → Anna S."

---

## 7. Migration Plan

Since the roster system is already built and has data, this needs a migration:

### 7.1 Database Migration

```sql
-- Migration: roster_schedule_v2

-- 1. Add new columns to rosters
ALTER TABLE rosters
    ADD COLUMN schedule_weeks_ahead INTEGER NOT NULL DEFAULT 12,
    ADD COLUMN max_consecutive_weeks INTEGER NOT NULL DEFAULT 2,
    ADD COLUMN handoff_day INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN active_hours_start TIME,
    ADD COLUMN active_hours_end TIME;

-- 2. Modify roster_members: drop position, add is_active tracking
ALTER TABLE roster_members
    DROP CONSTRAINT IF EXISTS roster_members_roster_id_position_key,
    DROP COLUMN IF EXISTS position,
    ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN left_at TIMESTAMPTZ;

-- 3. Create roster_schedule table
CREATE TABLE roster_schedule (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    roster_id         UUID NOT NULL REFERENCES rosters(id) ON DELETE CASCADE,
    week_start        DATE NOT NULL,
    week_end          DATE NOT NULL,
    primary_user_id   UUID REFERENCES users(id),
    secondary_user_id UUID REFERENCES users(id),
    is_locked         BOOLEAN NOT NULL DEFAULT false,
    generated         BOOLEAN NOT NULL DEFAULT true,
    notes             TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(roster_id, week_start),
    CHECK (primary_user_id IS DISTINCT FROM secondary_user_id),
    CHECK (week_end > week_start)
);

CREATE INDEX idx_roster_schedule_roster ON roster_schedule(roster_id, week_start);
CREATE INDEX idx_roster_schedule_current ON roster_schedule(roster_id, week_start, week_end);

-- 4. Backfill: convert existing calculated rotation to explicit schedule entries
-- (This should be done by Go migration code, not raw SQL, because it needs
--  the rotation calculation logic to determine who was on-call for past weeks)
```

### 7.2 Implementation Order

1. Run database migration (add new table and columns)
2. Backfill existing rosters: generate schedule from current members using the new algorithm
3. Update backend: new schedule endpoints, modified on-call resolution
4. Update worker: schedule top-up cron, updated escalation on-call lookup
5. Update frontend: new roster detail layout with schedule table
6. Update iCal export to use schedule table
7. Update Slack `/nightowl oncall` to use new resolution logic
8. Update demo seed data to include schedule entries

---

## 8. Implementation Prompt for Claude Code

```
Read docs/08-roster-v2.md and implement the roster v2 schedule system:

1. Database migration: add roster_schedule table, modify rosters and 
   roster_members per the migration plan in section 7.1

2. Schedule generation algorithm (internal/roster/scheduler.go):
   - Fair round-robin with max_consecutive_weeks constraint
   - Respects locked weeks
   - Tracks primary_weeks_served for fairness

3. New API endpoints: schedule CRUD, generate, lock/unlock per section 4

4. Update on-call resolution: override → schedule → unassigned

5. Update worker: weekly schedule top-up cron job

6. Update frontend roster detail page: schedule table with edit/lock, 
   member management with active/inactive, fairness report

7. Update iCal export and Slack oncall command

8. Update seed-demo data to include schedule entries

9. Backfill migration: convert any existing roster data to schedule entries

Build, test, fix, and commit when green.
```

---

## 9. Acceptance Criteria

- [ ] Schedule table shows 12 weeks of future primary/secondary assignments
- [ ] Clicking edit on a week opens dialog to change primary/secondary
- [ ] Edited weeks are locked and survive regeneration
- [ ] Regenerate button reassigns all unlocked future weeks fairly
- [ ] Adding/removing members triggers regeneration of unlocked weeks
- [ ] Overrides still take precedence over schedule for specific date ranges
- [ ] "Who's on call now" resolves: override → schedule → unassigned
- [ ] Fairness report shows even distribution over time
- [ ] iCal export reflects schedule (not calculated rotation)
- [ ] Slack `/nightowl oncall` uses new resolution
- [ ] Dashboard widget shows current week's schedule
- [ ] Past weeks are dimmed and not editable
- [ ] Empty roster (0 members) shows warning, not error
