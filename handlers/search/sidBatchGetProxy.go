package Search

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"opt360-portal-backend/config"

	"github.com/gin-gonic/gin"
)

type sidBatchGetRequest struct {
	SIDs []string `json:"sids"`
}

type sidLookupResult struct {
	SID        string `json:"sid"`
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code"`
	Response   any    `json:"response,omitempty"`
	Error      string `json:"error,omitempty"`
}

func GetSIDBatchValues(c *gin.Context) {
	var req sidBatchGetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	if len(req.SIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sids are required"})
		return
	}

	log.Printf("[GetSIDBatchValues] Processing %d SID lookups", len(req.SIDs))

	cfg, err := config.LoadConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load configuration", "details": err.Error()})
		return
	}

	baseURL := strings.TrimRight(cfg.SIDStore.BaseURL, "/")
	client := &http.Client{Timeout: time.Duration(cfg.SIDStore.TimeoutSeconds) * time.Second}
	results := make([]sidLookupResult, len(req.SIDs))

	var wg sync.WaitGroup
	for i, sid := range req.SIDs {
		i, sid := i, strings.TrimSpace(sid)
		if sid == "" {
			results[i] = sidLookupResult{SID: sid, Success: false, StatusCode: http.StatusBadRequest, Error: "sid is required"}
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			endpoint := fmt.Sprintf("%s/api/opt_details/sid/%s", baseURL, url.PathEscape(sid))

			upstreamReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, endpoint, nil)
			if err != nil {
				results[i] = sidLookupResult{SID: sid, Success: false, StatusCode: http.StatusInternalServerError, Error: "failed to create upstream request"}
				return
			}

			upstreamResp, err := client.Do(upstreamReq)
			if err != nil {
				results[i] = sidLookupResult{SID: sid, Success: false, StatusCode: http.StatusBadGateway, Error: err.Error()}
				return
			}
			defer upstreamResp.Body.Close()

			respBytes, err := io.ReadAll(upstreamResp.Body)
			if err != nil {
				results[i] = sidLookupResult{SID: sid, Success: false, StatusCode: http.StatusBadGateway, Error: "failed to read upstream response"}
				return
			}

			var parsed any
			if len(respBytes) > 0 {
				if err := json.Unmarshal(respBytes, &parsed); err != nil {
					parsed = gin.H{"raw": string(respBytes)}
				}
			}

			results[i] = sidLookupResult{
				SID:        sid,
				Success:    upstreamResp.StatusCode >= 200 && upstreamResp.StatusCode < 300,
				StatusCode: upstreamResp.StatusCode,
				Response:   parsed,
			}
		}()
	}
	wg.Wait()

	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	log.Printf("[GetSIDBatchValues] Completed: %d/%d successful", successCount, len(req.SIDs))
	c.JSON(http.StatusOK, gin.H{
		"success":    successCount == len(req.SIDs),
		"message":    "SID lookups completed",
		"requested":  len(req.SIDs),
		"successful": successCount,
		"results":    results,
	})
}
