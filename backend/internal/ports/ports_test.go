package ports_test

import (
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/ports"

	"github.com/stretchr/testify/assert"
)

func TestListParamsLimit(t *testing.T) {
	tests := []struct {
		name    string
		perPage int
		want    int
	}{
		{"zero defaults to 25", 0, 25},
		{"negative defaults to 25", -10, 25},
		{"within range passes through", 10, 10},
		{"exactly 1 passes through", 1, 1},
		{"exactly 100 passes through", 100, 100},
		{"above 100 clamps to 100", 150, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ports.ListParams{PerPage: tt.perPage}
			assert.Equal(t, tt.want, p.Limit())
		})
	}
}

func TestListParamsOffset(t *testing.T) {
	tests := []struct {
		name    string
		page    int
		perPage int
		want    int
	}{
		{"page zero defaults to page 1 -> offset 0", 0, 25, 0},
		{"negative page defaults to page 1 -> offset 0", -3, 25, 0},
		{"page 1 -> offset 0 regardless of perPage", 1, 10, 0},
		{"page 3 with perPage 10 -> offset 20", 3, 10, 20},
		{"page 2 with default perPage (25) -> offset 25", 2, 0, 25},
		{"page 2 with perPage clamped to 100 -> offset 100", 2, 150, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ports.ListParams{Page: tt.page, PerPage: tt.perPage}
			assert.Equal(t, tt.want, p.Offset())
		})
	}
}

func TestListParamsOffsetDoesNotMutateReceiver(t *testing.T) {
	// Offset/Limit take ListParams by value, so calling them must never
	// affect the caller's copy of Page/PerPage.
	p := ports.ListParams{Page: 0, PerPage: 0}
	_ = p.Offset()
	assert.Equal(t, 0, p.Page)
	assert.Equal(t, 0, p.PerPage)
}

func TestJobOptions(t *testing.T) {
	t.Run("WithQueue sets Queue", func(t *testing.T) {
		o := &ports.JobOptions{}
		ports.WithQueue("emails")(o)
		assert.Equal(t, "emails", o.Queue)
	})

	t.Run("WithMaxRetry sets MaxRetry", func(t *testing.T) {
		o := &ports.JobOptions{}
		ports.WithMaxRetry(7)(o)
		assert.Equal(t, 7, o.MaxRetry)
	})

	t.Run("WithProcessIn sets ProcessIn", func(t *testing.T) {
		o := &ports.JobOptions{}
		ports.WithProcessIn(5 * time.Minute)(o)
		assert.Equal(t, 5*time.Minute, o.ProcessIn)
	})

	t.Run("WithUniqueTTL sets UniqueTTL", func(t *testing.T) {
		o := &ports.JobOptions{}
		ports.WithUniqueTTL(30 * time.Second)(o)
		assert.Equal(t, 30*time.Second, o.UniqueTTL)
	})

	t.Run("composed options apply independently onto the same struct", func(t *testing.T) {
		o := &ports.JobOptions{}
		opts := []ports.JobOption{
			ports.WithQueue("renewals"),
			ports.WithMaxRetry(3),
			ports.WithProcessIn(time.Hour),
			ports.WithUniqueTTL(2 * time.Hour),
		}
		for _, apply := range opts {
			apply(o)
		}
		assert.Equal(t, ports.JobOptions{
			Queue:     "renewals",
			MaxRetry:  3,
			ProcessIn: time.Hour,
			UniqueTTL: 2 * time.Hour,
		}, *o)
	})
}
