package models

// used for High,low,Med risk operators list response

type OperatorListResponse struct {
	Data []Operator `json:"data"`
}

type Operator struct {
	OptID          string  `json:"Opt_id"`
	OptRiskScore   float64 `json:"Opt_risk_score"`
	ActiveStatus   int     `json:"Active_status"`
	OptName        string  `json:"Opt_name"`
	OptDistrict    string  `json:"Opt_district"`
	OptState       string  `json:"Opt_state"`
	OptRO          string  `json:"Opt_ro"`
	LastSyncTime   int64   `json:"Last_sync_time"`
	OptEA          string  `json:"Opt_ea"`
	OptReg         string  `json:"Opt_reg"`
	PacketsPerDay  float64 `json:"Packets_per_day"`
}