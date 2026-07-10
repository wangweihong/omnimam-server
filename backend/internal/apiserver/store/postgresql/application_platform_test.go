package postgresql

import (
	stderrors "errors"
	"strings"
	"testing"

	toolerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

func TestApplicationPlatformOwnerNameIndexesMatchContract(t *testing.T) {
	if !strings.Contains(applicationPlatformOwnerNameIndexesSQL, "DROP INDEX IF EXISTS idx_aiapp_app_templates_owner_name") {
		t.Fatalf("template index repair SQL must drop stale index: %s", applicationPlatformOwnerNameIndexesSQL)
	}
	if !strings.Contains(
		applicationPlatformOwnerNameIndexesSQL,
		"ON aiapp_app_templates(owner_user_id, name)",
	) {
		t.Fatalf("template index must use owner_user_id + name: %s", applicationPlatformOwnerNameIndexesSQL)
	}
	if !strings.Contains(applicationPlatformOwnerNameIndexesSQL, "DROP INDEX IF EXISTS idx_aiapp_app_engines_owner_name") {
		t.Fatalf("app engine index repair SQL must drop stale index: %s", applicationPlatformOwnerNameIndexesSQL)
	}
	if !strings.Contains(
		applicationPlatformOwnerNameIndexesSQL,
		"ON aiapp_app_engines(owner_user_id, name)",
	) {
		t.Fatalf("app engine index must use owner_user_id + name: %s", applicationPlatformOwnerNameIndexesSQL)
	}
}

func TestMapApplicationPlatformUniqueError(t *testing.T) {
	err := stderrors.New(
		`ERROR: duplicate key value violates unique constraint "idx_aiapp_app_templates_owner_name" (SQLSTATE 23505)`,
	)
	got := mapApplicationPlatformUniqueError(err, map[string]int{
		"idx_aiapp_app_templates_owner_name": code.ErrTemplateNameDuplicated,
	}, "template name duplicated")
	status := toolerrors.ToStatus(got)
	if status.Code != code.ErrTemplateNameDuplicated {
		t.Fatalf("code = %d, want %d", status.Code, code.ErrTemplateNameDuplicated)
	}

	other := stderrors.New(`ERROR: duplicate key value violates unique constraint "idx_other" (SQLSTATE 23505)`)
	if got := mapApplicationPlatformUniqueError(other, map[string]int{
		"idx_aiapp_app_templates_owner_name": code.ErrTemplateNameDuplicated,
	}, "template name duplicated"); got != other {
		t.Fatalf("unmatched constraint should return original error")
	}
}
