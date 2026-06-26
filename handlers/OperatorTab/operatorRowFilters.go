package OperatorTab

import (
	"fmt"
	"strings"
)

// operatorListFilters holds the optional query-param filters accepted by
// GetOperatorList.
type operatorListFilters struct {
	OptEa, OptReg, OptDistrict, OptState, OptID, ActiveStatus, Risk string
}

// filterOperatorRows applies f to rows (parquet rows decoded as
// map[string]interface{}), dropping empty rows and any row that fails a
// requested filter.
func filterOperatorRows(rows []map[string]interface{}, f operatorListFilters) []map[string]interface{} {
	filtered := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		if matchesOperatorFilters(row, f) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func matchesOperatorFilters(row map[string]interface{}, f operatorListFilters) bool {
	if f.OptState != "" && !rowFieldEqualFold(row, "opt_state", f.OptState) {
		return false
	}
	if f.OptID != "" && !rowFieldContainsFold(row, "opt_id", f.OptID) {
		return false
	}
	if f.OptEa != "" && !rowFieldEqualFold(row, "opt_ea", f.OptEa) {
		return false
	}
	if f.OptReg != "" && !rowFieldEqualFold(row, "opt_reg", f.OptReg) {
		return false
	}
	if f.OptDistrict != "" && !rowFieldEqualFold(row, "opt_district", f.OptDistrict) {
		return false
	}
	if f.ActiveStatus != "" && !matchesActiveStatus(row, f.ActiveStatus) {
		return false
	}
	if f.Risk != "" && !matchesRiskBucket(row, f.Risk) {
		return false
	}
	return true
}

// findRowField looks up a parquet row field by name, case-insensitively.
func findRowField(row map[string]interface{}, key string) (interface{}, bool) {
	for k, v := range row {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return nil, false
}

func rowFieldEqualFold(row map[string]interface{}, key, want string) bool {
	val, ok := findRowField(row, key)
	if !ok {
		return false
	}
	return strings.EqualFold(fmt.Sprintf("%v", val), want)
}

func rowFieldContainsFold(row map[string]interface{}, key, want string) bool {
	val, ok := findRowField(row, key)
	if !ok {
		return false
	}
	return strings.Contains(strings.ToLower(fmt.Sprintf("%v", val)), strings.ToLower(want))
}

// matchesActiveStatus maps "active"/"inactive" to the row's numeric
// Active_status field. Any other value is treated as "no filter" (matches
// everything), matching the original handler's behavior.
func matchesActiveStatus(row map[string]interface{}, activeStatus string) bool {
	var expected int
	switch {
	case strings.EqualFold(activeStatus, "active"):
		expected = 1
	case strings.EqualFold(activeStatus, "inactive"):
		expected = 0
	default:
		return true
	}

	val, ok := findRowField(row, "active_status")
	if !ok {
		return false
	}
	switch v := val.(type) {
	case int:
		return v == expected
	case float64:
		return int(v) == expected
	case string:
		return v == fmt.Sprintf("%v", expected)
	default:
		return false
	}
}

// matchesRiskBucket buckets the row's Opt_risk_score into high (>0.7),
// med (0.4-0.7), or low (<0.4) and compares against the requested bucket.
func matchesRiskBucket(row map[string]interface{}, risk string) bool {
	val, ok := findRowField(row, "opt_risk_score")
	if !ok {
		return false
	}

	var score float64
	switch v := val.(type) {
	case float64:
		score = v
	case float32:
		score = float64(v)
	case int:
		score = float64(v)
	case string:
		fmt.Sscanf(v, "%f", &score)
	default:
		return false
	}

	switch {
	case strings.EqualFold(risk, "high"):
		return score > 0.7
	case strings.EqualFold(risk, "med"):
		return score >= 0.4 && score <= 0.7
	case strings.EqualFold(risk, "low"):
		return score < 0.4
	default:
		return false
	}
}
