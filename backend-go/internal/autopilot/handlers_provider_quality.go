package autopilot

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleProviderQualityBudget GET /api/health-center/provider-quality/budget
// 返回当前进程的手动 L3 探测预算，不触发任何上游请求。
func handleProviderQualityBudget(probe *ProviderQualityProbe) gin.HandlerFunc {
	return func(c *gin.Context) {
		if probe == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "ProviderQuality 探测器不可用",
				"code":  "probe_unavailable",
			})
			return
		}
		c.JSON(http.StatusOK, probe.BudgetState())
	}
}

// handleProviderQualityProbe POST /api/health-center/provider-quality/probe
// 执行固定 canary；请求和响应均不包含明文 API Key 或模型原始输出。
func handleProviderQualityProbe(probe *ProviderQualityProbe) gin.HandlerFunc {
	return func(c *gin.Context) {
		if probe == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "ProviderQuality 探测器不可用",
				"code":  "probe_unavailable",
			})
			return
		}
		var req ProviderQualityProbeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "请求体格式无效",
				"code":  "invalid_json",
			})
			return
		}

		result, err := probe.Probe(c.Request.Context(), req)
		if err == nil {
			c.JSON(http.StatusOK, result)
			return
		}

		var probeErr *ProviderQualityProbeError
		if errors.As(err, &probeErr) {
			response := gin.H{
				"error": probeErr.Message,
				"code":  probeErr.Code,
			}
			if probe != nil && probeErr.Code == "probe_budget_exhausted" {
				response["budget"] = probe.BudgetState()
			}
			c.JSON(probeErr.HTTPStatus, response)
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "ProviderQuality 探测失败",
			"code":  "probe_failed",
		})
	}
}
