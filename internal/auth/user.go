package auth

import "context"

type User struct {
	ID          string
	Name        string
	DisplayName string
}

type contextKey struct{}

func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(contextKey{}).(*User)
	return u
}

func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, contextKey{}, u)
}
