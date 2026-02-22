package roster

import (
	"fmt"
	"strings"
	"time"
)

// generateICSFromSchedule produces an iCal feed from explicit roster_schedule entries.
func generateICSFromSchedule(roster RosterResponse, schedule []ScheduleEntry, overrides []OverrideResponse) string {
	var b strings.Builder

	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//NightOwl//Roster//EN\r\n")
	b.WriteString(fmt.Sprintf("X-WR-CALNAME:%s On-Call\r\n", roster.Name))
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")

	tz, err := time.LoadLocation(roster.Timezone)
	if err != nil {
		tz = time.UTC
	}

	handoffTime, err := time.Parse("15:04", roster.HandoffTime)
	if err != nil {
		handoffTime, _ = time.Parse("15:04", "09:00")
	}

	for _, entry := range schedule {
		weekStart, err := time.Parse("2006-01-02", entry.WeekStart)
		if err != nil {
			continue
		}
		weekEnd, err := time.Parse("2006-01-02", entry.WeekEnd)
		if err != nil {
			continue
		}

		shiftStart := time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(),
			handoffTime.Hour(), handoffTime.Minute(), 0, 0, tz)
		shiftEnd := time.Date(weekEnd.Year(), weekEnd.Month(), weekEnd.Day(),
			handoffTime.Hour(), handoffTime.Minute(), 0, 0, tz)

		primaryName := "Unassigned"
		if entry.PrimaryDisplayName != "" {
			primaryName = entry.PrimaryDisplayName
		}

		uid := fmt.Sprintf("%s-%s@nightowl", roster.ID, entry.WeekStart)
		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
		b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", shiftStart.UTC().Format("20060102T150405Z")))
		b.WriteString(fmt.Sprintf("DTEND:%s\r\n", shiftEnd.UTC().Format("20060102T150405Z")))
		b.WriteString(fmt.Sprintf("SUMMARY:On-Call: %s\r\n", primaryName))

		desc := fmt.Sprintf("Roster: %s\\nPrimary: %s", roster.Name, primaryName)
		if entry.SecondaryDisplayName != "" {
			desc += fmt.Sprintf("\\nSecondary: %s", entry.SecondaryDisplayName)
		}
		if entry.Notes != nil && *entry.Notes != "" {
			desc += fmt.Sprintf("\\nNotes: %s", *entry.Notes)
		}
		b.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", desc))
		b.WriteString("END:VEVENT\r\n")
	}

	// Add overrides as separate events.
	for _, o := range overrides {
		uid := fmt.Sprintf("override-%s@nightowl", o.ID)
		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
		b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", o.StartAt.UTC().Format("20060102T150405Z")))
		b.WriteString(fmt.Sprintf("DTEND:%s\r\n", o.EndAt.UTC().Format("20060102T150405Z")))
		b.WriteString(fmt.Sprintf("SUMMARY:Override: %s\r\n", o.DisplayName))
		reason := ""
		if o.Reason != nil {
			reason = *o.Reason
		}
		b.WriteString(fmt.Sprintf("DESCRIPTION:Override on %s\\nReason: %s\r\n", roster.Name, reason))
		b.WriteString("END:VEVENT\r\n")
	}

	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}
