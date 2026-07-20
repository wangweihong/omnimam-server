package postgresql

import (
	"strings"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestCompileAssetSelectorUsesParameterizedConditions(t *testing.T) {
	expression := &iapiserver.AssetSelectorExpression{Operator: "and", Children: []*iapiserver.AssetSelectorExpression{
		{Operator: "predicate", Predicate: &iapiserver.AssetSelectorPredicate{Kind: "label", Key: "character.name", Action: "eq", Values: []string{"alice"}}},
		{Operator: "predicate", Predicate: &iapiserver.AssetSelectorPredicate{Kind: "tag", Action: "neq", Values: []string{"draft"}}},
	}}
	condition, args, err := compileAssetSelector(expression, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if condition == "" || len(args) != 5 {
		t.Fatalf("condition=%q args=%#v", condition, args)
	}
	for _, value := range []string{"alice", "draft", "user-1"} {
		if contains := strings.Contains(condition, value); contains {
			t.Fatalf("condition interpolated untrusted value %q: %s", value, condition)
		}
	}
}
