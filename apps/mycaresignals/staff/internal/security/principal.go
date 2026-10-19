package security

import "context"

// Principal is an identity verified and constructed inside this package. Its
// identity fields are immutable to callers and it deliberately contains no
// roles; authorization loads current roles for every decision.
type Principal struct {
	staffUserID     int64
	externalSubject string
	email           string
	displayName     string
	source          string
}

func newPrincipal(staffUserID int64, externalSubject, email, displayName, source string) (Principal, bool) {
	principal := Principal{
		staffUserID:     staffUserID,
		externalSubject: externalSubject,
		email:           email,
		displayName:     displayName,
		source:          source,
	}
	return principal, principal.valid()
}

func (p Principal) StaffUserID() int64      { return p.staffUserID }
func (p Principal) ExternalSubject() string { return p.externalSubject }
func (p Principal) Email() string           { return p.email }
func (p Principal) DisplayName() string     { return p.displayName }
func (p Principal) Source() string          { return p.source }

func (p Principal) valid() bool {
	return p.staffUserID > 0 && p.externalSubject != "" && p.source != ""
}

type principalContextKey struct{}

// WithPrincipal stores a value copy. External packages cannot manufacture a
// valid Principal because all identity fields and its constructor are private.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	if !principal.valid() {
		return ctx
	}
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// PrincipalFromContext returns a value copy of the verified principal.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok && principal.valid()
}
