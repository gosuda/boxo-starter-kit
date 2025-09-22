package security

import "context"

// UserInfo represents authenticated user information
type UserInfo struct {
	ID       string
	Username string
	Scope    string
	Claims   map[string]interface{}
}

type contextKey string

const userInfoKey contextKey = "userInfo"

// WithUserInfo adds user information to context
func WithUserInfo(ctx context.Context, user *UserInfo) context.Context {
	return context.WithValue(ctx, userInfoKey, user)
}

// GetUserInfo retrieves user information from context
func GetUserInfo(ctx context.Context) *UserInfo {
	if user, ok := ctx.Value(userInfoKey).(*UserInfo); ok {
		return user
	}
	return nil
}

// IsAuthenticated checks if user is authenticated
func IsAuthenticated(ctx context.Context) bool {
	return GetUserInfo(ctx) != nil
}

// HasScope checks if user has required scope
func HasScope(ctx context.Context, requiredScope string) bool {
	user := GetUserInfo(ctx)
	return user != nil && user.Scope == requiredScope
}

// IsAdmin checks if user is an admin
func IsAdmin(ctx context.Context, adminUsers []string) bool {
	user := GetUserInfo(ctx)
	if user == nil {
		return false
	}

	for _, admin := range adminUsers {
		if user.Username == admin {
			return true
		}
	}
	return false
}
