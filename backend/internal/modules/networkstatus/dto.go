package networkstatus

import (
	"time"
)

// IssueInput creates a network status entry. Empty Type/Severity/Status fall
// back to the table defaults (issue/minor/investigating); a zero StartsAt is
// filled with the current time by the service.
type IssueInput struct {
	Title    string     `json:"title" validate:"required,min=2,max=200"`
	Body     string     `json:"body"`
	Type     string     `json:"type"`     // "" or one of scheduled|issue|outage
	Severity string     `json:"severity"` // "" or one of minor|major|critical
	Status   string     `json:"status"`   // "" or investigating|identified|monitoring|resolved|scheduled
	Affected string     `json:"affected"`
	StartsAt *time.Time `json:"starts_at"`
	EndsAt   *time.Time `json:"ends_at"`
}

// IssueUpdateInput patches a network status entry; nil fields are left
// unchanged. ClearEnds wins over EndsAt and blanks the resolution time.
type IssueUpdateInput struct {
	Title     *string    `json:"title" validate:"omitempty,min=2,max=200"`
	Body      *string    `json:"body"`
	Type      *string    `json:"type"`
	Severity  *string    `json:"severity"`
	Status    *string    `json:"status"`
	Affected  *string    `json:"affected"`
	StartsAt  *time.Time `json:"starts_at"`
	EndsAt    *time.Time `json:"ends_at"`
	ClearEnds bool       `json:"clear_ends"`
}
