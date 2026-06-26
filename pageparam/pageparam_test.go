package pageparam

import "testing"

func TestSlice(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7}

	cases := []struct {
		name       string
		page, size int
		wantItems  []int
		wantPage   int
		wantTotal  int
	}{
		{"first page", 1, 3, []int{1, 2, 3}, 1, 3},
		{"middle page", 2, 3, []int{4, 5, 6}, 2, 3},
		{"last partial page", 3, 3, []int{7}, 3, 3},
		{"page beyond range clamps to last", 5, 3, []int{7}, 3, 3},
		{"page size larger than total", 1, 100, items, 1, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Slice(items, tc.page, tc.size)
			if len(got.Items) != len(tc.wantItems) {
				t.Fatalf("got %v items, want %v", got.Items, tc.wantItems)
			}
			for i := range got.Items {
				if got.Items[i] != tc.wantItems[i] {
					t.Fatalf("got %v, want %v", got.Items, tc.wantItems)
				}
			}
			if got.Page != tc.wantPage {
				t.Errorf("got page %d, want %d", got.Page, tc.wantPage)
			}
			if got.TotalPages != tc.wantTotal {
				t.Errorf("got totalPages %d, want %d", got.TotalPages, tc.wantTotal)
			}
		})
	}
}

func TestSliceEmpty(t *testing.T) {
	got := Slice([]int{}, 1, 10)
	if len(got.Items) != 0 {
		t.Fatalf("expected empty result, got %v", got.Items)
	}
	if got.TotalPages != 0 {
		t.Errorf("expected 0 total pages, got %d", got.TotalPages)
	}
}
