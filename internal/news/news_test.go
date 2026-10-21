package news

import (
	"slices"
	"testing"
)

func TestCategories(t *testing.T) {
	want := []Category{
		{"business", "Business"},
		{"education", "Education"},
		{"environment", "Environment"},
		{"food", "Food"},
		{"health", "Health"},
		{"lifestyle", "Lifestyle"},
		{"science", "Science"},
		{"sports", "Sports"},
		{"technology", "Technology"},
	}
	got := Categories()
	if !slices.Equal(got, want) {
		t.Fatalf("Categories() = %v, want %v", got, want)
	}

	got[0] = Category{Slug: "mutated"}
	if again := Categories(); again[0].Slug != "business" {
		t.Errorf("Categories() shares its backing array: first slug is now %q", again[0].Slug)
	}
}

func TestParseCategory(t *testing.T) {
	tests := []struct {
		raw    string
		want   Category
		wantOK bool
	}{
		{"business", Category{"business", "Business"}, true},
		{"Business", Category{"business", "Business"}, true},
		{"BUSINESS", Category{"business", "Business"}, true},
		{"sports", Category{"sports", "Sports"}, true},
		{"politics", Category{}, false},
		{"../x", Category{}, false},
		{"ſports", Category{}, false}, // long s: lowercasing keeps it, so it must not match "sports"
		{"", Category{}, false},
		{" business", Category{}, false},
		{"business ", Category{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, ok := ParseCategory(tt.raw)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("ParseCategory(%q) = (%+v, %v), want (%+v, %v)", tt.raw, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
