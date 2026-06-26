package LandingPage

import (
	"log"
	"net/http"
	"sort"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/config"
	"opt360-portal-backend/respond"
	"opt360-portal-backend/s3store"

	"github.com/gin-gonic/gin"
)

type SelectedEAsRequest struct {
	SelectedEAs []string `json:"selected_eas" binding:"required"`
}

type EADistribution struct {
	HighRisk int `json:"high_risk"`
	MedRisk  int `json:"med_risk"`
	LowRisk  int `json:"low_risk"`
	NoRisk   int `json:"no_risk"`
}

type AuditData struct {
	EADistribution map[string]EADistribution `json:"ea_distribution"`
}

func loadAuditData(c *gin.Context, logPrefix, regionalOffice, adID string) (AuditData, bool) {
	s3Cfg := config.GetDefaultS3Config()
	key := s3store.OperatorFilePath(regionalOffice, "audit.json")

	var auditData AuditData
	if err := s3store.FetchJSON(s3Cfg, key, &auditData); err != nil {
		if s3store.IsNotFound(err) {
			log.Printf("%s S3 fetch failed key=%s user=%s: %v", logPrefix, key, adID, err)
			respond.Error(c, http.StatusNotFound, "Audit file not found", err, gin.H{
				"regional_office": regionalOffice,
				"file_path":       key,
			})
		} else {
			log.Printf("%s Parse failed key=%s user=%s: %v", logPrefix, key, adID, err)
			respond.Error(c, http.StatusInternalServerError, "Failed to parse audit data", err, nil)
		}
		return AuditData{}, false
	}
	return auditData, true
}

func GetSelectedEAs(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	var req SelectedEAsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[GetSelectedEAs] Invalid request body user=%s: %v", user.ADID, err)
		respond.Error(c, http.StatusBadRequest, "Invalid request body", err, nil)
		return
	}
	if len(req.SelectedEAs) == 0 {
		respond.Error(c, http.StatusBadRequest, "selected_eas array cannot be empty", nil, nil)
		return
	}

	auditData, ok := loadAuditData(c, "[GetSelectedEAs]", user.RegionalOffice, user.ADID)
	if !ok {
		return
	}

	filteredDistribution := make(map[string]EADistribution)
	for _, eaName := range req.SelectedEAs {
		if distribution, exists := auditData.EADistribution[eaName]; exists {
			filteredDistribution[eaName] = distribution
		}
	}

	log.Printf("[GetSelectedEAs] Returning %d EAs for user=%s", len(filteredDistribution), user.ADID)
	respond.OK(c, gin.H{"ea_distribution": filteredDistribution})
}

func GetTop10EAs(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	auditData, ok := loadAuditData(c, "[GetTop10EAs]", user.RegionalOffice, user.ADID)
	if !ok {
		return
	}

	type eaWithTotal struct {
		name         string
		distribution EADistribution
		totalRisk    int
	}
	var eaList []eaWithTotal
	for eaName, distribution := range auditData.EADistribution {
		totalRisk := distribution.LowRisk + distribution.MedRisk + distribution.HighRisk
		eaList = append(eaList, eaWithTotal{name: eaName, distribution: distribution, totalRisk: totalRisk})
	}
	sort.Slice(eaList, func(i, j int) bool { return eaList[i].totalRisk > eaList[j].totalRisk })

	top10Count := 10
	if len(eaList) < 10 {
		top10Count = len(eaList)
	}
	top10Distribution := make(map[string]EADistribution)
	for i := 0; i < top10Count; i++ {
		top10Distribution[eaList[i].name] = eaList[i].distribution
	}

	log.Printf("[GetTop10EAs] Returning top %d EAs for user=%s", top10Count, user.ADID)
	respond.OK(c, gin.H{"ea_distribution": top10Distribution})
}

func GetAllEAs(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	auditData, ok := loadAuditData(c, "[GetAllEAs]", user.RegionalOffice, user.ADID)
	if !ok {
		return
	}

	eaNames := make([]string, 0, len(auditData.EADistribution))
	for eaName := range auditData.EADistribution {
		eaNames = append(eaNames, eaName)
	}
	sort.Strings(eaNames)

	log.Printf("[GetAllEAs] Returning %d EAs for user=%s", len(eaNames), user.ADID)
	respond.OK(c, gin.H{"eas": eaNames, "count": len(eaNames)})
}
