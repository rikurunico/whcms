package announcements

import (
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
)

// AnnouncementInput creates an announcement. Slug is optional and derived from
// the title when omitted.
type AnnouncementInput struct {
	Title     string `json:"title" validate:"required,min=2,max=200"`
	Slug      string `json:"slug" validate:"omitempty,min=2,max=200"`
	Body      string `json:"body"`
	Published bool   `json:"published"`
}

// AnnouncementUpdateInput patches an announcement; nil fields are left
// unchanged.
type AnnouncementUpdateInput struct {
	Title     *string `json:"title" validate:"omitempty,min=2,max=200"`
	Slug      *string `json:"slug" validate:"omitempty,min=2,max=200"`
	Body      *string `json:"body"`
	Published *bool   `json:"published"`
}

// AnnouncementResponse is the API representation of an announcement (the
// soft-delete marker is never exposed).
type AnnouncementResponse struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Body        string     `json:"body"`
	Published   bool       `json:"published"`
	PublishedAt *time.Time `json:"published_at"`
	AuthorID    *int64     `json:"author_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// toResponse maps a domain announcement to its API representation.
func toResponse(a domain.Announcement) AnnouncementResponse {
	return AnnouncementResponse{
		ID:          a.ID,
		Title:       a.Title,
		Slug:        a.Slug,
		Body:        a.Body,
		Published:   a.Published,
		PublishedAt: a.PublishedAt,
		AuthorID:    a.AuthorID,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
}

// toResponses maps a slice of announcements, always returning a non-nil slice
// (the FE expects an array).
func toResponses(list []domain.Announcement) []AnnouncementResponse {
	out := make([]AnnouncementResponse, 0, len(list))
	for _, a := range list {
		out = append(out, toResponse(a))
	}
	return out
}
