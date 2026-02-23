package messaging

import "fmt"

// SeverityEmoji returns the emoji prefix for a given severity level.
func SeverityEmoji(severity string) string {
	switch severity {
	case "critical":
		return "\U0001F534" // red circle
	case "major":
		return "\U0001F7E0" // orange circle
	case "warning":
		return "\U0001F7E1" // yellow circle
	case "info":
		return "\U0001F535" // blue circle
	default:
		return "\u26AA" // white circle
	}
}

// SeverityLabel returns a human-readable uppercase label for a severity.
func SeverityLabel(s string) string {
	switch s {
	case "critical":
		return "CRITICAL"
	case "major":
		return "MAJOR"
	case "warning":
		return "WARNING"
	case "info":
		return "INFO"
	default:
		return s
	}
}

// AlertSummary builds a one-line text summary for an alert.
func AlertSummary(msg AlertMessage) string {
	return fmt.Sprintf("%s %s: %s", SeverityEmoji(msg.Severity), SeverityLabel(msg.Severity), msg.Title)
}

// SeverityColor returns a hex color string for a severity level.
func SeverityColor(severity string) string {
	switch severity {
	case "critical":
		return "#DC2626"
	case "major":
		return "#EA580C"
	case "warning":
		return "#CA8A04"
	case "info":
		return "#2563EB"
	default:
		return "#6B7280"
	}
}

// Truncate returns s truncated to max characters with "..." appended.
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
