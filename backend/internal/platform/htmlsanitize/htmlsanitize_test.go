package htmlsanitize_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsdlamongan/whcms/backend/internal/platform/htmlsanitize"
)

func TestHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text passes through", "hello world", "hello world"},
		{"safe formatting tags survive", "<p>hi</p><b>bold</b>", "<p>hi</p><b>bold</b>"},
		{"script tag stripped", `<p>hi</p><script>alert(document.cookie)</script>`, "<p>hi</p>"},
		{"inline event handler stripped", `<img src=x onerror=alert(1)>`, `<img src="x">`},
		{"javascript: link neutralized", `<a href="javascript:alert(1)">click</a>`, "click"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, htmlsanitize.HTML(tt.in))
		})
	}
}
