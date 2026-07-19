//go:build integration

package postgresql

import (
	"context"
	"errors"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

func TestPostgresComfyUIObjectInfoCurrentFact(t *testing.T) {
	dsn := os.Getenv("OMNIMAM_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("OMNIMAM_TEST_POSTGRES_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&iapiserver.ComfyUIEngineObjectInfo{}, &iapiserver.EngineInstance{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(&iapiserver.ComfyUIEngineObjectInfo{}, &iapiserver.EngineInstance{})
	})
	if err := db.AutoMigrate(&iapiserver.EngineInstance{}, &iapiserver.ComfyUIEngineObjectInfo{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`ALTER TABLE aiapp_comfyui_engine_object_info ADD CONSTRAINT fk_test_comfyui_object_info_engine FOREIGN KEY (engine_instance_id) REFERENCES aiapp_engine_instances(id) ON DELETE CASCADE`).Error; err != nil {
		t.Fatal(err)
	}

	storage := newApplicationPlatform(&datastore{db: db})
	engine := &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", BaseURL: "http://127.0.0.1:8188", AuthType: "none", AuthConfig: map[string]any{}, Enabled: true, HealthStatus: iapiserver.EngineHealthOnline, MaxConcurrency: 1, RequestTimeoutSeconds: 5, TaskTimeoutSeconds: 30}
	engine.Name = "object-info-integration"
	if _, err := storage.AddEngineInstance(context.Background(), engine); err != nil {
		t.Fatal(err)
	}

	first, err := storage.RefreshComfyUIEngineObjectInfo(context.Background(), engine.ID, func(*iapiserver.EngineInstance) (*iapiserver.ComfyUIEngineObjectInfo, error) {
		return &iapiserver.ComfyUIEngineObjectInfo{ObjectInfo: map[string]any{"KSampler": map[string]any{"input": map[string]any{}, "output": []any{}}}, ComfyUIVersion: "v1", RefreshedAt: imachinery.Now()}, nil
	})
	if err != nil || first.ComfyUIVersion != "v1" {
		t.Fatalf("first refresh=%#v err=%v", first, err)
	}

	refreshFailure := errors.New("controlled refresh failure")
	if _, err := storage.RefreshComfyUIEngineObjectInfo(context.Background(), engine.ID, func(*iapiserver.EngineInstance) (*iapiserver.ComfyUIEngineObjectInfo, error) {
		return nil, refreshFailure
	}); !errors.Is(err, refreshFailure) {
		t.Fatalf("refresh error=%v, want controlled failure", err)
	}
	persisted, err := storage.GetComfyUIEngineObjectInfo(context.Background(), engine.ID)
	if err != nil || persisted.ComfyUIVersion != "v1" || persisted.ObjectInfo["KSampler"] == nil {
		t.Fatalf("last successful catalog was not retained: catalog=%#v err=%v", persisted, err)
	}

	if err := storage.DeleteEngineInstance(context.Background(), engine.ID); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&iapiserver.ComfyUIEngineObjectInfo{}).Where("engine_instance_id = ?", engine.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("catalog rows after engine deletion=%d, want 0", count)
	}
}
