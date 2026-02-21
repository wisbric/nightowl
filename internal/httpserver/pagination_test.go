package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCursorRoundTrip(t *testing.T) {
	original := Cursor{
		CreatedAt: time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
		ID:        uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
	}

	encoded := EncodeCursor(original)
	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor() error = %v", err)
	}

	if !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", decoded.CreatedAt, original.CreatedAt)
	}
	if decoded.ID != original.ID {
		t.Errorf("ID = %v, want %v", decoded.ID, original.ID)
	}
}

func TestDecodeCursor_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"not base64", "!!!invalid!!!"},
		{"missing colon", "MTIzNDU2"},
		{"bad timestamp", "YWJjOjU1MGU4NDAwLWUyOWItNDFkNC1hNzE2LTQ0NjY1NTQ0MDAwMA"},
		{"bad uuid", "MTIzNDU2Nzg5MDpub3QtYS11dWlk"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeCursor(tt.input)
			if err == nil {
				t.Errorf("DecodeCursor(%q) should return error", tt.input)
			}
		})
	}
}

func TestParseCursorParams(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantLimit int
		wantAfter bool
		wantErr   bool
	}{
		{
			name:      "defaults",
			query:     "",
			wantLimit: DefaultPageSize,
			wantAfter: false,
		},
		{
			name:      "custom limit",
			query:     "limit=50",
			wantLimit: 50,
		},
		{
			name:      "limit capped at max",
			query:     "limit=500",
			wantLimit: MaxPageSize,
		},
		{
			name:    "negative limit",
			query:   "limit=-1",
			wantErr: true,
		},
		{
			name:    "non-numeric limit",
			query:   "limit=abc",
			wantErr: true,
		},
		{
			name:    "invalid cursor",
			query:   "after=invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil)
			p, err := ParseCursorParams(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCursorParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if p.Limit != tt.wantLimit {
				t.Errorf("Limit = %d, want %d", p.Limit, tt.wantLimit)
			}
			if (p.After != nil) != tt.wantAfter {
				t.Errorf("After present = %v, want %v", p.After != nil, tt.wantAfter)
			}
		})
	}
}

func TestParseCursorParams_WithValidCursor(t *testing.T) {
	c := Cursor{
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		ID:        uuid.New(),
	}
	encoded := EncodeCursor(c)

	r := httptest.NewRequest(http.MethodGet, "/?after="+encoded+"&limit=10", nil)
	p, err := ParseCursorParams(r)
	if err != nil {
		t.Fatalf("ParseCursorParams() error = %v", err)
	}
	if p.After == nil {
		t.Fatal("After should not be nil")
	}
	if !p.After.CreatedAt.Equal(c.CreatedAt) {
		t.Errorf("After.CreatedAt = %v, want %v", p.After.CreatedAt, c.CreatedAt)
	}
	if p.Limit != 10 {
		t.Errorf("Limit = %d, want 10", p.Limit)
	}
}

func TestNewCursorPage(t *testing.T) {
	type item struct {
		ID        uuid.UUID
		CreatedAt time.Time
	}
	cursorFn := func(i item) Cursor {
		return Cursor{CreatedAt: i.CreatedAt, ID: i.ID}
	}

	t.Run("with more results", func(t *testing.T) {
		// Simulate fetching limit+1 items
		items := make([]item, 6)
		for i := range items {
			items[i] = item{ID: uuid.New(), CreatedAt: time.Now()}
		}

		page := NewCursorPage(items, 5, cursorFn)
		if len(page.Items) != 5 {
			t.Errorf("Items length = %d, want 5", len(page.Items))
		}
		if !page.HasMore {
			t.Error("HasMore should be true")
		}
		if page.NextCursor == nil {
			t.Error("NextCursor should not be nil")
		}
	})

	t.Run("without more results", func(t *testing.T) {
		items := make([]item, 3)
		for i := range items {
			items[i] = item{ID: uuid.New(), CreatedAt: time.Now()}
		}

		page := NewCursorPage(items, 5, cursorFn)
		if len(page.Items) != 3 {
			t.Errorf("Items length = %d, want 3", len(page.Items))
		}
		if page.HasMore {
			t.Error("HasMore should be false")
		}
		if page.NextCursor != nil {
			t.Error("NextCursor should be nil")
		}
	})

	t.Run("empty results", func(t *testing.T) {
		var items []item
		page := NewCursorPage(items, 5, cursorFn)
		if len(page.Items) != 0 {
			t.Errorf("Items length = %d, want 0", len(page.Items))
		}
		if page.HasMore {
			t.Error("HasMore should be false")
		}
	})
}

func TestParseOffsetParams(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		wantPage     int
		wantPageSize int
		wantOffset   int
		wantErr      bool
	}{
		{
			name:         "defaults",
			query:        "",
			wantPage:     1,
			wantPageSize: DefaultPageSize,
			wantOffset:   0,
		},
		{
			name:         "custom page and size",
			query:        "page=3&page_size=10",
			wantPage:     3,
			wantPageSize: 10,
			wantOffset:   20,
		},
		{
			name:         "page_size capped",
			query:        "page_size=500",
			wantPageSize: MaxPageSize,
			wantPage:     1,
			wantOffset:   0,
		},
		{
			name:    "negative page",
			query:   "page=-1",
			wantErr: true,
		},
		{
			name:    "zero page",
			query:   "page=0",
			wantErr: true,
		},
		{
			name:    "non-numeric page_size",
			query:   "page_size=abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil)
			p, err := ParseOffsetParams(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOffsetParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if p.Page != tt.wantPage {
				t.Errorf("Page = %d, want %d", p.Page, tt.wantPage)
			}
			if p.PageSize != tt.wantPageSize {
				t.Errorf("PageSize = %d, want %d", p.PageSize, tt.wantPageSize)
			}
			if p.Offset != tt.wantOffset {
				t.Errorf("Offset = %d, want %d", p.Offset, tt.wantOffset)
			}
		})
	}
}

func TestNewOffsetPage(t *testing.T) {
	type item struct{ Name string }

	tests := []struct {
		name           string
		itemCount      int
		params         OffsetParams
		totalItems     int
		wantTotalPages int
	}{
		{
			name:           "first of multiple pages",
			itemCount:      10,
			params:         OffsetParams{Page: 1, PageSize: 10},
			totalItems:     25,
			wantTotalPages: 3,
		},
		{
			name:           "single page",
			itemCount:      3,
			params:         OffsetParams{Page: 1, PageSize: 10},
			totalItems:     3,
			wantTotalPages: 1,
		},
		{
			name:           "exact fit",
			itemCount:      10,
			params:         OffsetParams{Page: 1, PageSize: 10},
			totalItems:     10,
			wantTotalPages: 1,
		},
		{
			name:           "empty",
			itemCount:      0,
			params:         OffsetParams{Page: 1, PageSize: 10},
			totalItems:     0,
			wantTotalPages: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := make([]item, tt.itemCount)
			page := NewOffsetPage(items, tt.params, tt.totalItems)

			if len(page.Items) != tt.itemCount {
				t.Errorf("Items length = %d, want %d", len(page.Items), tt.itemCount)
			}
			if page.TotalPages != tt.wantTotalPages {
				t.Errorf("TotalPages = %d, want %d", page.TotalPages, tt.wantTotalPages)
			}
			if page.TotalItems != tt.totalItems {
				t.Errorf("TotalItems = %d, want %d", page.TotalItems, tt.totalItems)
			}
		})
	}
}
