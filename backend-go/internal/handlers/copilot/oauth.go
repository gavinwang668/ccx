package copilot

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	corecopilot "github.com/BenedictKing/ccx/internal/copilot"
	"github.com/BenedictKing/ccx/internal/httpclient"
	"github.com/gin-gonic/gin"
)

var oauthRequestTimeout = 30 * time.Second
var newOAuthClient = corecopilot.NewOAuthClient

type tokenRequest struct {
	DeviceCode string `json:"deviceCode"`
	ProxyURL   string `json:"proxyUrl"`
}

type verifyRequest struct {
	AccessToken string `json:"accessToken"`
	ProxyURL    string `json:"proxyUrl"`
}

type deviceCodeRequest struct {
	ProxyURL string `json:"proxyUrl"`
}

func oauthErrorMessage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline exceeded") {
		return "GitHub OAuth 请求超时：无法连接 github.com，请检查网络或代理后重试"
	}
	return err.Error()
}

func oauthHTTPClient(proxyURL string) *http.Client {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return nil
	}
	return httpclient.GetManager().NewStandardClient(oauthRequestTimeout, false, proxyURL)
}

func bindOptionalOAuthRequest(c *gin.Context, out interface{}) bool {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return true
	}
	if err := c.ShouldBindJSON(out); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return false
	}
	return true
}

// RequestDeviceCode 发起 GitHub Copilot OAuth Device Flow。
func RequestDeviceCode() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req deviceCodeRequest
		if !bindOptionalOAuthRequest(c, &req) {
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), oauthRequestTimeout)
		defer cancel()

		log.Printf("[Copilot-OAuth] 请求 GitHub device code: ip=%s timeout=%s", c.ClientIP(), oauthRequestTimeout)
		client := newOAuthClient(oauthHTTPClient(req.ProxyURL))
		deviceCode, err := client.RequestDeviceCode(ctx)
		if err != nil {
			log.Printf("[Copilot-OAuth] GitHub device code 请求失败: ip=%s error=%v", c.ClientIP(), err)
			c.JSON(http.StatusBadGateway, gin.H{"error": oauthErrorMessage(err)})
			return
		}

		log.Printf("[Copilot-OAuth] GitHub device code 请求成功: ip=%s expiresIn=%d interval=%d", c.ClientIP(), deviceCode.ExpiresIn, deviceCode.Interval)
		c.JSON(http.StatusOK, gin.H{
			"deviceCode":      deviceCode.DeviceCode,
			"userCode":        deviceCode.UserCode,
			"verificationUri": deviceCode.VerificationURI,
			"expiresIn":       deviceCode.ExpiresIn,
			"interval":        deviceCode.Interval,
		})
	}
}

// PollAccessToken 轮询 GitHub OAuth access token。
func PollAccessToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req tokenRequest
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.DeviceCode) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "deviceCode is required"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), oauthRequestTimeout)
		defer cancel()

		client := newOAuthClient(oauthHTTPClient(req.ProxyURL))
		token, err := client.PollAccessToken(ctx, req.DeviceCode)
		if err != nil {
			log.Printf("[Copilot-OAuth] GitHub access token 轮询失败: ip=%s error=%v", c.ClientIP(), err)
			c.JSON(http.StatusBadGateway, gin.H{"error": oauthErrorMessage(err)})
			return
		}
		if token.Error != "" {
			c.JSON(http.StatusOK, gin.H{
				"error":            token.Error,
				"errorDescription": token.ErrorDescription,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"accessToken": token.AccessToken,
			"tokenType":   token.TokenType,
			"scope":       token.Scope,
		})
	}
}

// VerifyToken 验证 GitHub OAuth token 并返回用户信息。
func VerifyToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req verifyRequest
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.AccessToken) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "accessToken is required"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), oauthRequestTimeout)
		defer cancel()

		client := newOAuthClient(oauthHTTPClient(req.ProxyURL))
		user, err := client.VerifyUser(ctx, req.AccessToken)
		if err != nil {
			log.Printf("[Copilot-OAuth] GitHub token 验证失败: ip=%s error=%v", c.ClientIP(), err)
			c.JSON(http.StatusBadRequest, gin.H{"error": oauthErrorMessage(err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"login":     user.Login,
			"id":        user.ID,
			"avatarUrl": user.AvatarURL,
			"htmlUrl":   user.HTMLURL,
		})
	}
}
