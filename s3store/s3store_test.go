package s3store

import "testing"

func TestOperatorFilePath(t *testing.T) {
	cases := []struct {
		ro, file, want string
	}{
		{"Uttar Pradesh", "operator_high.parquet", "opt360Store/UttarPradesh/operator_high.parquet"},
		{"DELHI", "kpi.json", "opt360Store/Delhi/kpi.json"},
		{"delhi", "kpi.json", "opt360Store/Delhi/kpi.json"},
	}

	for _, tc := range cases {
		got := OperatorFilePath(tc.ro, tc.file)
		if got != tc.want {
			t.Errorf("OperatorFilePath(%q, %q) = %q, want %q", tc.ro, tc.file, got, tc.want)
		}
	}
}
