package roster

import (
	"fmt"
	"strings"
	"time"
)

// generateICS produces an iCal feed for a roster's rotation schedule.
// It generates events for the next 30 days and includes overrides.
func generateICS(roster RosterResponse, members []MemberResponse, overrides []OverrideResponse) string {
	var b strings.Builder

	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//NightOwl//Roster//EN\r\n")
	b.WriteString(fmt.Sprintf("X-WR-CALNAME:%s On-Call\r\n", roster.Name))
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")

	if len(members) == 0 {
		b.WriteString("END:VCALENDAR\r\n")
		return b.String()
	}

	tz, err := time.LoadLocation(roster.Timezone)
	if err != nil {
		tz = time.UTC
	}

	handoffTime, err := time.Parse("15:04", roster.HandoffTime)
	if err != nil {
		handoffTime, _ = time.Parse("15:04", "09:00")
	}

	startDate, err := time.Parse("2006-01-02", roster.StartDate)
	if err != nil {
		startDate = time.Now()
	}

	rotationDays := 7 // default weekly
	if roster.RotationType == "daily" {
		rotationDays = 1
	}
	if roster.RotationLength > 0 {
		rotationDays = int(roster.RotationLength)
	}

	// Generate shift events for the next 30 days.
	now := time.Now().In(tz)
	rangeStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, tz)
	rangeEnd := rangeStart.AddDate(0, 0, 30)

	for d := rangeStart; d.Before(rangeEnd); d = d.AddDate(0, 0, rotationDays) {
		shiftStart := time.Date(d.Year(), d.Month(), d.Day(),
			handoffTime.Hour(), handoffTime.Minute(), 0, 0, tz)
		shiftEnd := shiftStart.AddDate(0, 0, rotationDays)

		// Calculate which member is on-call.
		daysSinceStart := int(d.Sub(startDate).Hours() / 24)
		if daysSinceStart < 0 {
			daysSinceStart = 0
		}
		cycle := daysSinceStart / rotationDays
		pos := cycle % len(members)
		member := members[pos]

		uid := fmt.Sprintf("%s-%s@nightowl", roster.ID, shiftStart.Format("20060102"))
		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
		b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", shiftStart.UTC().Format("20060102T150405Z")))
		b.WriteString(fmt.Sprintf("DTEND:%s\r\n", shiftEnd.UTC().Format("20060102T150405Z")))
		b.WriteString(fmt.Sprintf("SUMMARY:On-Call: %s\r\n", member.UserID))
		b.WriteString(fmt.Sprintf("DESCRIPTION:Roster: %s\\nPosition: %d\r\n", roster.Name, member.Position))
		b.WriteString("END:VEVENT\r\n")
	}

	// Add overrides as separate events.
	for _, o := range overrides {
		uid := fmt.Sprintf("override-%s@nightowl", o.ID)
		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
		b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", o.StartAt.UTC().Format("20060102T150405Z")))
		b.WriteString(fmt.Sprintf("DTEND:%s\r\n", o.EndAt.UTC().Format("20060102T150405Z")))
		b.WriteString(fmt.Sprintf("SUMMARY:Override: %s\r\n", o.UserID))
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
