package security_test

import (
	"context"
	"reflect"
	"testing"

	"therapeutic-engine/internal/security"
)

func TestExternalPackagesCannotPopulatePrincipalIdentity(t *testing.T) {
	typ := reflect.TypeOf(security.Principal{})
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).IsExported() {
			t.Fatalf("Principal identity field %s is externally writable", typ.Field(i).Name)
		}
	}
	ctx := security.WithPrincipal(context.Background(), security.Principal{})
	if _, ok := security.PrincipalFromContext(ctx); ok {
		t.Fatal("zero-value Principal authenticated")
	}
}
