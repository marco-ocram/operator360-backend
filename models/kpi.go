package models

type KPIResponse struct {
	HighRiskCount            int                `json:"high_risk_count"`
	HighestRiskOpt           HighestRiskOperator`json:"highest_risk_opt"`
	LowRiskCount             int                `json:"low_risk_count"`
	MedRiskCount             int                `json:"med_risk_count"`
	MostCommonAnomalyCode    string             `json:"most_common_anomaly_code"`
	MostCommonAnomalyGroup   string             `json:"most_common_anomaly_group"`
	MostCommonAnomalyName    string             `json:"most_common_anomaly_name"`
	OptRO                    string             `json:"opt_ro"`
	Top10Operators           []TopOperator      `json:"top_10_opt"`
}


type HighestRiskOperator struct {
	OptID      string  `json:"opt_id"`
	OptName    string  `json:"opt_name"`
	RiskScore  float64 `json:"risk_score"`
}


type TopOperator struct {
	ActiveStatus   int     `json:"active_status"`
	LastSyncTime   int64   `json:"last_sync_time"`
	OptDistrict    string  `json:"opt_district"`
	OptEA          string  `json:"opt_ea"`
	OptID          string  `json:"opt_id"`
	OptName        string  `json:"opt_name"`
	OptReg         string  `json:"opt_reg"`
	OptRiskScore   float64 `json:"opt_risk_score"`
	OptRO          string  `json:"opt_ro"`
	OptState       string  `json:"opt_state"`
	PacketsPerDay  float64  `json:"packets_per_day"`
	Rank           int     `json:"rn"`
}