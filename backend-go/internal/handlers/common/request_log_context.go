package common

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/BenedictKing/ccx/internal/scheduler"
	"github.com/gin-gonic/gin"
)

const (
	requestLogContextKey = "requestLogContext"
	consoleJSONTextLimit = 1000
)

type httpRequestLogContextKey struct{}

type RequestLogContext struct {
	SessionID string
	Round     int
}

func SetRequestLogContext(c *gin.Context, sessionID string, round int) {
	if c == nil {
		return
	}
	c.Set(requestLogContextKey, RequestLogContext{
		SessionID: strings.TrimSpace(sessionID),
		Round:     round,
	})
}

func RequestLogTag(c *gin.Context) string {
	ctx, ok := requestLogContextFromGin(c)
	if !ok {
		return ""
	}
	return requestLogTag(ctx)
}

func WithRequestLogContext(req *http.Request, c *gin.Context) *http.Request {
	if req == nil {
		return nil
	}
	ctx, ok := requestLogContextFromGin(c)
	if !ok {
		return req
	}
	return req.WithContext(context.WithValue(req.Context(), httpRequestLogContextKey{}, ctx))
}

func requestLogTagFromRequest(req *http.Request) string {
	if req == nil {
		return ""
	}
	ctx, ok := req.Context().Value(httpRequestLogContextKey{}).(RequestLogContext)
	if !ok {
		return ""
	}
	return requestLogTag(ctx)
}

func requestLogContextFromGin(c *gin.Context) (RequestLogContext, bool) {
	if c == nil {
		return RequestLogContext{}, false
	}
	value, ok := c.Get(requestLogContextKey)
	if !ok {
		return RequestLogContext{}, false
	}
	ctx, ok := value.(RequestLogContext)
	if !ok {
		return RequestLogContext{}, false
	}
	return ctx, true
}

func requestLogTag(ctx RequestLogContext) string {
	parts := make([]string, 0, 2)
	if ctx.SessionID != "" {
		parts = append(parts, "session="+scheduler.MaskUserIDForLog(ctx.SessionID))
	}
	if ctx.Round > 0 {
		parts = append(parts, fmt.Sprintf("round=%d", ctx.Round))
	}
	if len(parts) == 0 {
		return ""
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func RequestLogf(c *gin.Context, format string, args ...interface{}) {
	tag := RequestLogTag(c)
	logWithTag(tag, format, args...)
}

func LogWithTag(tag string, format string, args ...interface{}) {
	logWithTag(tag, format, args...)
}

func requestLogToConsole(c *gin.Context, format string, args ...interface{}) {
	tag := RequestLogTag(c)
	if tag == "" {
		logToConsole(format, args...)
		return
	}
	logToConsole(taggedFormat(format, tag), args...)
}

func requestLogToFile(c *gin.Context, format string, args ...interface{}) {
	tag := RequestLogTag(c)
	if tag == "" {
		logToFile(format, args...)
		return
	}
	logToFile(taggedFormat(format, tag), args...)
}

func requestLogToConsoleFromRequest(req *http.Request, format string, args ...interface{}) {
	tag := requestLogTagFromRequest(req)
	if tag == "" {
		logToConsole(format, args...)
		return
	}
	logToConsole(taggedFormat(format, tag), args...)
}

func requestLogToFileFromRequest(req *http.Request, format string, args ...interface{}) {
	tag := requestLogTagFromRequest(req)
	if tag == "" {
		logToFile(format, args...)
		return
	}
	logToFile(taggedFormat(format, tag), args...)
}

func logWithTag(tag string, format string, args ...interface{}) {
	if tag == "" {
		log.Printf(format, args...)
		return
	}
	log.Printf(taggedFormat(format, tag), args...)
}

func taggedFormat(format string, tag string) string {
	if tag == "" {
		return format
	}
	if strings.HasPrefix(format, "[") {
		if idx := strings.Index(format, "]"); idx >= 0 {
			return format[:idx+1] + " " + tag + format[idx+1:]
		}
	}
	return tag + " " + format
}
