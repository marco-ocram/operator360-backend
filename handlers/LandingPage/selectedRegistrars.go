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

type SelectedRegistrarsRequest struct {
	SelectedRegistrars []string `json:"selected_registrars" binding:"required"`
}

type RegistrarDistribution struct {
	HighRisk int `json:"high_risk"`
	MedRisk  int `json:"med_risk"`
	LowRisk  int `json:"low_risk"`
	NoRisk   int `json:"no_risk"`
}

type AuditDataRegistrar struct {
	RegDistribution map[string]RegistrarDistribution `json:"reg_distribution"`
}

func loadAuditDataRegistrar(c *gin.Context, logPrefix, regionalOffice, adID string) (AuditDataRegistrar, bool) {
	s3Cfg := config.GetDefaultS3Config()
	key := s3store.OperatorFilePath(regionalOffice, "audit.json")

	var auditData AuditDataRegistrar
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
		return AuditDataRegistrar{}, false
	}
	return auditData, true
}

func GetSelectedRegistrars(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	var req SelectedRegistrarsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[GetSelectedRegistrars] Invalid request body user=%s: %v", user.ADID, err)
		respond.Error(c, http.StatusBadRequest, "Invalid request body", err, nil)
		return
	}
	if len(req.SelectedRegistrars) == 0 {
		respond.Error(c, http.StatusBadRequest, "selected_registrars array cannot be empty", nil, nil)
		return
	}

	auditData, ok := loadAuditDataRegistrar(c, "[GetSelectedRegistrars]", user.RegionalOffice, user.ADID)
	if !ok {
		return
	}

	filteredDistribution := make(map[string]RegistrarDistribution)
	for _, registrarName := range req.SelectedRegistrars {
		if distribution, exists := auditData.RegDistribution[registrarName]; exists {
			filteredDistribution[registrarName] = distribution
		}
	}

	log.Printf("[GetSelectedRegistrars] Returning %d registrars for user=%s", len(filteredDistribution), user.ADID)
	respond.OK(c, gin.H{"reg_distribution": filteredDistribution})
}

func GetTop10Registrars(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	auditData, ok := loadAuditDataRegistrar(c, "[GetTop10Registrars]", user.RegionalOffice, user.ADID)
	if !ok {
		return
	}

	type registrarWithTotal struct {
		name         string
		distribution RegistrarDistribution
		totalRisk    int
	}
	var registrarList []registrarWithTotal
	for registrarName, distribution := range auditData.RegDistribution {
		totalRisk := distribution.LowRisk + distribution.MedRisk + distribution.HighRisk
		registrarList = append(registrarList, registrarWithTotal{name: registrarName, distribution: distribution, totalRisk: totalRisk})
	}
	sort.Slice(registrarList, func(i, j int) bool { return registrarList[i].totalRisk > registrarList[j].totalRisk })

	top10Count := 10
	if len(registrarList) < 10 {
		top10Count = len(registrarList)
	}
	top10Distribution := make(map[string]RegistrarDistribution)
	for i := 0; i < top10Count; i++ {
		top10Distribution[registrarList[i].name] = registrarList[i].distribution
	}

	log.Printf("[GetTop10Registrars] Returning top %d registrars for user=%s", top10Count, user.ADID)
	respond.OK(c, gin.H{"reg_distribution": top10Distribution})
}

func GetAllRegistrars(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	auditData, ok := loadAuditDataRegistrar(c, "[GetAllRegistrars]", user.RegionalOffice, user.ADID)
	if !ok {
		return
	}

	registrarNames := make([]string, 0, len(auditData.RegDistribution))
	for registrarName := range auditData.RegDistribution {
		registrarNames = append(registrarNames, registrarName)
	}
	sort.Strings(registrarNames)

	log.Printf("[GetAllRegistrars] Returning %d registrars for user=%s", len(registrarNames), user.ADID)
	respond.OK(c, gin.H{"registrars": registrarNames, "count": len(registrarNames)})
}
