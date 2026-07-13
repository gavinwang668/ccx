package autopilot

import "context"

type requestProfileContextKey struct{}

// ContextWithRequestProfile 将请求画像按值写入 context，避免下游修改共享指针。
func ContextWithRequestProfile(ctx context.Context, profile RequestProfile) context.Context {
	return context.WithValue(ctx, requestProfileContextKey{}, profile)
}

// RequestProfileFromContext 返回当前请求画像的副本。
func RequestProfileFromContext(ctx context.Context) (RequestProfile, bool) {
	if ctx == nil {
		return RequestProfile{}, false
	}
	profile, ok := ctx.Value(requestProfileContextKey{}).(RequestProfile)
	return profile, ok
}
