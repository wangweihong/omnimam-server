package ctxvalue_test

import (
	"context"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

func TestGetValue(t *testing.T) {
	ctx := context.Background()
	d := &iapiserver.User{}
	d.Name = "test"
	ctx = context.WithValue(ctx, "user", d)

	v, err := ctxvalue.GetValue[*iapiserver.User](ctx, "user")
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != d.Name {
		t.Fatal("not match")
	}

	_, err = ctxvalue.GetValue[string](ctx, "user")
	if err == nil {
		t.Fatal("wrong type match")
	}

}
