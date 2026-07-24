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

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestPostgresRequiredEngineBindings(t *testing.T) {
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

	schema := fmt.Sprintf("test_aiapp_system_binding_%d", time.Now().UnixNano())
	if err := tx.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec("SET LOCAL search_path TO " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.AutoMigrate(&iapiserver.EngineInstance{}, &iapiserver.EngineCapabilityBinding{}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`CREATE UNIQUE INDEX idx_aiapp_binding_engine_capability ON aiapp_engine_capability_bindings(engine_instance_id, provider_capability_id)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`ALTER TABLE aiapp_engine_capability_bindings ADD CONSTRAINT fk_aiapp_binding_engine FOREIGN KEY (engine_instance_id) REFERENCES aiapp_engine_instances(id)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(applicationPlatformBindingCascadeSQL).Error; err != nil {
		t.Fatal(err)
	}
	var deleteAction string
	if err := tx.Raw(`SELECT confdeltype::text FROM pg_constraint WHERE conname = 'fk_aiapp_binding_engine' AND conrelid = 'aiapp_engine_capability_bindings'::regclass`).Scan(&deleteAction).Error; err != nil {
		t.Fatal(err)
	}
	if deleteAction != "c" {
		t.Fatalf("binding foreign-key delete action=%q, want cascade", deleteAction)
	}

	storage := newApplicationPlatform(&datastore{db: tx})
	ctx := context.Background()
	failedEngine := integrationEngine("rollback-engine", "comfyui")
	duplicateBindings := []*iapiserver.EngineCapabilityBinding{
		integrationBinding("comfyui-workflow-runtime", "old"),
		integrationBinding("comfyui-workflow-runtime", "old"),
	}
	if _, err := storage.AddEngineInstanceWithBindings(ctx, failedEngine, duplicateBindings); err == nil {
		t.Fatal("duplicate required bindings did not fail")
	}
	var failedCount int64
	if err := tx.Model(&iapiserver.EngineInstance{}).Where("name = ?", failedEngine.Name).Count(&failedCount).Error; err != nil {
		t.Fatal(err)
	}
	if failedCount != 0 {
		t.Fatalf("rolled back engine count=%d", failedCount)
	}

	comfy := integrationEngine("comfy-engine", "comfyui")
	deepseek := integrationEngine("deepseek-engine", "deepseek_official")
	if _, err := storage.AddEngineInstance(ctx, comfy); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.AddEngineInstance(ctx, deepseek); err != nil {
		t.Fatal(err)
	}
	if err := storage.EnsureRequiredEngineBindings(ctx, "comfyui", "comfyui-workflow-runtime", "2026-07-24.1", "ComfyUI Workflow Runtime", "System-managed required capability binding"); err != nil {
		t.Fatal(err)
	}

	var bindings []*iapiserver.EngineCapabilityBinding
	if err := tx.Order("engine_instance_id ASC").Find(&bindings).Error; err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 || bindings[0].EngineInstanceID != comfy.ID || bindings[0].ProviderCapabilityRevision != "2026-07-24.1" || !bindings[0].Enabled || len(bindings[0].Restrictions) != 0 {
		t.Fatalf("unexpected reconciled bindings: %#v", bindings)
	}

	binding := bindings[0]
	if err := tx.Model(&iapiserver.EngineCapabilityBinding{}).Where("id = ?", binding.ID).Updates(map[string]any{
		"provider_capability_revision": "stale",
		"enabled":                      false,
		"restrictions_json":            `{"model_ids":["stale"]}`,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := storage.EnsureRequiredEngineBindings(ctx, "comfyui", "comfyui-workflow-runtime", "2026-07-24.1", "ComfyUI Workflow Runtime", "System-managed required capability binding"); err != nil {
		t.Fatal(err)
	}
	var repaired iapiserver.EngineCapabilityBinding
	if err := tx.First(&repaired, "id = ?", binding.ID).Error; err != nil {
		t.Fatal(err)
	}
	if repaired.ProviderCapabilityRevision != "2026-07-24.1" || !repaired.Enabled || len(repaired.Restrictions) != 0 || repaired.ResourceVersion <= binding.ResourceVersion {
		t.Fatalf("binding was not repaired: before=%#v after=%#v", binding, repaired)
	}
	stableVersion := repaired.ResourceVersion
	if err := storage.EnsureRequiredEngineBindings(ctx, "comfyui", "comfyui-workflow-runtime", "2026-07-24.1", "ComfyUI Workflow Runtime", "System-managed required capability binding"); err != nil {
		t.Fatal(err)
	}
	if err := tx.First(&repaired, "id = ?", binding.ID).Error; err != nil {
		t.Fatal(err)
	}
	if repaired.ResourceVersion != stableVersion {
		t.Fatalf("idempotent reconcile changed resource_version: %d -> %d", stableVersion, repaired.ResourceVersion)
	}

	if err := storage.DeleteEngineInstance(ctx, comfy.ID); err != nil {
		t.Fatal(err)
	}
	var bindingCount int64
	if err := tx.Model(&iapiserver.EngineCapabilityBinding{}).Where("engine_instance_id = ?", comfy.ID).Count(&bindingCount).Error; err != nil {
		t.Fatal(err)
	}
	if bindingCount != 0 {
		t.Fatalf("bindings after engine deletion=%d", bindingCount)
	}
}

func integrationEngine(name, engineType string) *iapiserver.EngineInstance {
	engine := &iapiserver.EngineInstance{
		ApplicationEngineTypeID: engineType,
		BaseURL:                 "http://127.0.0.1:8188",
		AuthType:                "none",
		AuthConfig:              map[string]any{},
		Enabled:                 true,
		HealthStatus:            iapiserver.EngineHealthUnknown,
		MaxConcurrency:          1,
		RequestTimeoutSeconds:   5,
		TaskTimeoutSeconds:      30,
	}
	engine.Name = name
	return engine
}

func integrationBinding(capabilityID, revision string) *iapiserver.EngineCapabilityBinding {
	binding := &iapiserver.EngineCapabilityBinding{
		ProviderCapabilityID:       capabilityID,
		ProviderCapabilityRevision: revision,
		Enabled:                    true,
		Restrictions:               map[string]any{},
	}
	binding.Name = capabilityID
	return binding
}
