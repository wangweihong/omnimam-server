//go:build integration

package postgresql

import (
	"os"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

func TestTaskCenterDAGObservabilityMigration(t *testing.T) {
	dsn := os.Getenv("OMNIMAM_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("OMNIMAM_TEST_POSTGRES_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if err := tx.AutoMigrate(
		&iapiserver.AtomicTask{},
		&iapiserver.DAGTaskGroup{},
		&iapiserver.TaskSchedule{},
		&iapiserver.TaskScheduleExecution{},
		&iapiserver.RuntimeProjectionEvent{},
	); err != nil {
		t.Fatal(err)
	}

	parent := migrationDAGGroup("parent")
	if err := tx.Create(parent).Error; err != nil {
		t.Fatal(err)
	}
	child := migrationDAGGroup("retry")
	child.RetryOfID = parent.ID
	if err := tx.Create(child).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&iapiserver.DAGTaskGroup{}).Where("id = ?", child.ID).Updates(map[string]any{
		"trigger_type":      "API",
		"triggered_at":      nil,
		"trigger_source_id": "",
	}).Error; err != nil {
		t.Fatal(err)
	}
	task := &iapiserver.AtomicTask{
		FunctionRef: "test.run", ChildKey: "render", OwnerType: iapiserver.TaskOwnerTypeDAGGroup, OwnerID: child.ID,
		Status: iapiserver.AtomicTaskStatusPending, ProjectID: "project", Namespace: "default", CreatedBy: "user-1",
		Arguments: map[string]any{}, Output: map[string]any{},
	}
	task.ID = uuid.NewString()
	task.Name = "legacy task"
	if err := tx.Create(task).Error; err != nil {
		t.Fatal(err)
	}

	if err := tx.Exec(taskCenterDAGObservabilityMigrationSQL).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(taskCenterDAGObservabilityMigrationSQL).Error; err != nil {
		t.Fatalf("migration is not idempotent: %v", err)
	}
	var migratedGroup iapiserver.DAGTaskGroup
	if err := tx.First(&migratedGroup, "id = ?", child.ID).Error; err != nil {
		t.Fatal(err)
	}
	if migratedGroup.TriggerType != iapiserver.DAGTriggerRetry || migratedGroup.TriggerSourceID != parent.ID || migratedGroup.TriggeredAt.IsZero() {
		t.Fatalf("migrated DAG trigger = type:%q source:%q at:%v", migratedGroup.TriggerType, migratedGroup.TriggerSourceID, migratedGroup.TriggeredAt)
	}
	var migratedTask iapiserver.AtomicTask
	if err := tx.First(&migratedTask, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if migratedTask.DAGNodeKey != task.ChildKey {
		t.Fatalf("dag_node_key = %q, want %q", migratedTask.DAGNodeKey, task.ChildKey)
	}
	for model, index := range map[any]string{
		&iapiserver.AtomicTask{}:             "idx_atomic_tasks_dag_node",
		&iapiserver.DAGTaskGroup{}:           "idx_dag_groups_status_time",
		&iapiserver.RuntimeProjectionEvent{}: "idx_runtime_projection_execution_time",
	} {
		if !tx.Migrator().HasIndex(model, index) {
			t.Fatalf("index %s was not created", index)
		}
	}
	if err := tx.Model(&iapiserver.DAGTaskGroup{}).Where("id = ?", child.ID).Update("trigger_type", "INVALID").Error; err == nil {
		t.Fatal("invalid trigger type unexpectedly satisfied the database constraint")
	}
}

func migrationDAGGroup(name string) *iapiserver.DAGTaskGroup {
	group := &iapiserver.DAGTaskGroup{
		Nodes: []iapiserver.DAGNode{}, Edges: []iapiserver.DAGEdge{}, Input: map[string]any{}, OutputMapping: map[string]any{},
		Status: iapiserver.TaskGroupStatusPending, Summary: iapiserver.TaskSummary{}, Result: map[string]any{},
		TriggerType: iapiserver.DAGTriggerAPI, TriggeredAt: imachinery.Now(), RuntimeDefinitionName: "migration_test",
		RuntimeDefinitionVersion: 1, RuntimeDefinitionHash: "hash", ProjectID: "project", Namespace: "default", CreatedBy: "user-1",
	}
	group.ID = uuid.NewString()
	group.Name = name
	return group
}
