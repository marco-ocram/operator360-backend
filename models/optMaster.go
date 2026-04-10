package models

import "time"

// OptMaster represents a row from data_platform.opt_master table
type OptMaster struct {
	ID                string     `json:"id"`
	UID               string     `json:"uid"`
	Name              string     `json:"name"`
	Phone             string     `json:"phone"`
	Email             string     `json:"email"`
	Reg               *string    `json:"reg"`
	RegCode           *string    `json:"reg_code"`
	EA                *string    `json:"ea"`
	EACode            *string    `json:"ea_code"`
	RO                *string    `json:"ro"`
	RiskScore         *float64   `json:"risk_score"`
	RiskBucket        *string    `json:"risk_bucket"`
	CenterName        *string    `json:"center_name"`
	CenterID          *string    `json:"center_id"`
	CenterAddress     *string    `json:"center_address"`
	Pincode           *string    `json:"pincode"`
	VTC               *string    `json:"vtc"`
	SubDistrict       *string    `json:"sub_district"`
	District          *string    `json:"district"`
	State             *string    `json:"state"`
	Latitude          *string    `json:"latitude"`
	Longitude         *string    `json:"longitude"`
	Altitude          *string    `json:"altitude"`
	MachineCode       *string    `json:"machine_code"`
	ClientVersion     *string    `json:"client_version"`
	ClientType        *string    `json:"client_type"`
	LastSyncTimestamp *time.Time `json:"last_sync_timestamp"`
	DataPath          *string    `json:"data_path"`
	UpdatedAt         *time.Time `json:"updated_at"`
}

// OptMasterListResponse wraps a paginated list of OptMaster records
type OptMasterListResponse struct {
	Data       []OptMaster `json:"data"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
	RiskBucket string      `json:"risk_bucket"`
}

// OperatorWithRisk is the slim projection returned by /operator_list_with_risk.
type OperatorWithRisk struct {
	ID                string     `json:"id"`
	UID               string     `json:"uid"`
	Name              string     `json:"name"`
	Phone             string     `json:"phone"`
	Email             string     `json:"email"`
	RiskScore         *float64   `json:"risk_score"`
	RiskBucket        *string    `json:"risk_bucket"`
	DataPath          *string    `json:"data_path"`
	UpdatedAt         *time.Time `json:"updated_at"`
	UserStatus        *string    `json:"user_status"`
	UserName          *string    `json:"user_name"`
	Reg               *string    `json:"reg"`
	EA                *string    `json:"ea"`
	District          *string    `json:"district"`
	State             *string    `json:"state"`
	LastSyncTimestamp *time.Time `json:"last_sync_timestamp"`
}

// OperatorWithRiskListResponse wraps a paginated list of OperatorWithRisk records.
type OperatorWithRiskListResponse struct {
	Data       []OperatorWithRisk `json:"data"`
	Total      int                `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
	TotalPages int                `json:"total_pages"`
	RiskBucket string             `json:"risk_bucket"`
}

// FilteredOperator is the slim projection returned by /filter_operator_list.
// UserStatus is a pointer so it is omitted from JSON when the query runs
// without the user_status filter (single-table path).
type FilteredOperator struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	RiskScore         *float64   `json:"risk_score"`
	RiskBucket        *string    `json:"risk_bucket"`
	Reg               *string    `json:"reg"`
	EA                *string    `json:"ea"`
	RO                *string    `json:"ro"`
	District          *string    `json:"district"`
	State             *string    `json:"state"`
	LastSyncTimestamp *time.Time `json:"last_sync_timestamp"`
	DataPath          *string    `json:"data_path"`
	UserStatus        *string    `json:"user_status"`
}

// FilteredOperatorAppliedFilters records which filters were active in the request.
type FilteredOperatorAppliedFilters struct {
	RO         string `json:"ro"`
	State      string `json:"state,omitempty"`
	District   string `json:"district,omitempty"`
	ID         string `json:"id,omitempty"`
	EA         string `json:"ea,omitempty"`
	Reg        string `json:"reg,omitempty"`
	RiskBucket string `json:"risk_bucket,omitempty"`
	UserStatus string `json:"user_status,omitempty"`
}

// FilteredOperatorResponse is the top-level response for /filter_operator_list.
type FilteredOperatorResponse struct {
	Data           []FilteredOperator             `json:"data"`
	Total          int                            `json:"total"`
	Page           int                            `json:"page"`
	PageSize       int                            `json:"page_size"`
	TotalPages     int                            `json:"total_pages"`
	AppliedFilters FilteredOperatorAppliedFilters `json:"applied_filters"`
}
