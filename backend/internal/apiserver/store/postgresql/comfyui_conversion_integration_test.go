//go:build integration

package postgresql

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

func TestPostgresComfyUIWorkflowRepeatedConversion(t *testing.T) {
	dsn := os.Getenv("OMNIMAM_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("OMNIMAM_TEST_POSTGRES_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })

	schema := fmt.Sprintf("test_comfy_conversion_%d", time.Now().UnixNano())
	if err := tx.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec("SET LOCAL search_path TO " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.AutoMigrate(&iapiserver.ComfyUIWorkflow{}, &iapiserver.ApplicationTemplate{}, &iapiserver.ApplicationTemplateVersion{}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`CREATE UNIQUE INDEX idx_aiapp_templates_conversion_key ON aiapp_application_templates(owner_user_id, comfyui_conversion_idempotency_key) WHERE comfyui_conversion_idempotency_key IS NOT NULL`).Error; err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	storage := newApplicationPlatform(&datastore{db: tx})
	workflow := integrationReadyWorkflow("workflow-1", "owner-1")
	if err := tx.Create(workflow).Error; err != nil {
		t.Fatal(err)
	}
	first := convertFixture("Template 1", "key-1", workflow.ID)
	firstResult, err := storage.ConvertComfyUIWorkflow(ctx, workflow.ID, workflow.OwnerUserID, "key-1", first.template, first.version)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := storage.GetComfyUIWorkflowConversion(ctx, workflow.ID, workflow.OwnerUserID, "key-1")
	if err != nil || replayed.ApplicationTemplate.ID != firstResult.ApplicationTemplate.ID {
		t.Fatalf("idempotent replay result=%#v err=%v", replayed, err)
	}

	second := convertFixture("Template 2", "key-2", workflow.ID)
	secondResult, err := storage.ConvertComfyUIWorkflow(ctx, workflow.ID, workflow.OwnerUserID, "key-2", second.template, second.version)
	if err != nil || secondResult.ApplicationTemplate.ID == firstResult.ApplicationTemplate.ID {
		t.Fatalf("second conversion result=%#v err=%v", secondResult, err)
	}

	other := integrationReadyWorkflow("workflow-2", "owner-1")
	if err := tx.Create(other).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := storage.GetComfyUIWorkflowConversion(ctx, other.ID, other.OwnerUserID, "key-1"); errors.ToStatus(err).Code != code.ErrAIAppComfyUIConversionIdempotencyConflict {
		t.Fatalf("cross-workflow key error=%v", err)
	}

	for _, column := range []string{"converted_application_template_id", "converted_template_version_id", "conversion_idempotency_key", "converted_at"} {
		var exists bool
		if err := tx.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'aiapp_comfyui_workflows' AND column_name = ?)`, column).Scan(&exists).Error; err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Fatalf("removed workflow column %s still exists", column)
		}
	}
}

type conversionFixture struct {
	template *iapiserver.ApplicationTemplate
	version  *iapiserver.ApplicationTemplateVersion
}

func convertFixture(name, key, workflowID string) conversionFixture {
	revision := "sha256:" + fmt.Sprintf("%064d", 1)
	template := &iapiserver.ApplicationTemplate{OwnerUserID: "owner-1", CapabilitySourceType: iapiserver.CapabilitySourceComfyUIWorkflow, CapabilityDefinitionID: "image.edit", ComfyUIConversionIdempotencyKey: &key}
	template.Name = name
	return conversionFixture{
		template: template,
		version:  &iapiserver.ApplicationTemplateVersion{Status: iapiserver.VersionStatusDraft, CapabilitySourceType: iapiserver.CapabilitySourceComfyUIWorkflow, SourceRevision: revision, WorkflowContractRevision: &revision, SourceComfyUIWorkflowID: &workflowID, TemplateContract: map[string]any{}, ComfyUIAPIWorkflow: map[string]any{"1": map[string]any{"class_type": "SaveImage", "inputs": map[string]any{}}}},
	}
}

func integrationReadyWorkflow(id, owner string) *iapiserver.ComfyUIWorkflow {
	checksum := "sha256:" + fmt.Sprintf("%064d", 1)
	workflow := &iapiserver.ComfyUIWorkflow{OwnerUserID: owner, CreatedByUserID: owner, UpdatedByUserID: owner, SourceType: iapiserver.ComfyUIWorkflowSourceAPI, APIConversionStatus: iapiserver.ComfyUIAPIConversionReady, APIWorkflow: map[string]any{"1": map[string]any{"class_type": "SaveImage", "inputs": map[string]any{}}}, SourceChecksum: checksum, APIWorkflowChecksum: &checksum}
	workflow.ID = id
	workflow.Name = id
	return workflow
}
