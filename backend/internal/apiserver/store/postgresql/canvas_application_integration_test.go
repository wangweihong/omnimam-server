//go:build integration

package postgresql

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

func TestCanvasApplicationOutboxAndArtifactProjection(t *testing.T) {
	dsn := os.Getenv("OMNIMAM_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("OMNIMAM_TEST_POSTGRES_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	models := []any{
		&iapiserver.Application{},
		&iapiserver.ApplicationVersion{},
		&iapiserver.ApplicationRun{},
		&iapiserver.ApplicationArtifactRef{},
		&iapiserver.AtomicTask{},
		&iapiserver.WorkflowCanvasRun{},
		&iapiserver.CanvasNodeRun{},
		&iapiserver.CanvasNodeRunTaskBinding{},
		&iapiserver.CanvasNodeRunOutputBinding{},
		&iapiserver.CanvasNodeRunFlowRef{},
		&iapiserver.WorkflowCanvasOutbox{},
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	ds := &datastore{db: db}
	if err := ds.ensureOutboxScheme(); err != nil {
		t.Fatal(err)
	}
	suffix := uuid.NewString()
	applicationID := "application-" + suffix
	versionID := "version-" + suffix
	applicationRunID := "application-run-" + suffix
	canvasRunID := "canvas-run-" + suffix
	nodeRunID := "node-run-" + suffix
	taskID := "task-" + suffix
	artifactID := "artifact-" + suffix
	defer cleanupCanvasApplicationIntegrationRows(
		t,
		db,
		suffix,
		applicationID,
		versionID,
		applicationRunID,
		canvasRunID,
		nodeRunID,
		taskID,
	)

	application := &iapiserver.Application{
		OwnerUserID:   "user-" + suffix,
		Visibility:    iapiserver.ApplicationVisibilityPrivate,
		RunEnabled:    true,
		CanvasEnabled: true,
	}
	application.ID = applicationID
	application.Name = "Application " + suffix
	if err := db.Create(application).Error; err != nil {
		t.Fatal(err)
	}
	version := &iapiserver.ApplicationVersion{
		ApplicationID:                applicationID,
		SemanticVersion:              "1.0.0-" + suffix,
		Status:                       iapiserver.VersionStatusDraft,
		ApplicationTemplateVersionID: "template-" + suffix,
		InputSchema:                  map[string]any{"type": "object", "properties": map[string]any{}},
		OutputSchema:                 map[string]any{"type": "object", "properties": map[string]any{}},
		ParameterPolicies:            map[string]any{},
	}
	version.ID = versionID
	version.Name = "Version " + suffix
	if err := db.Create(version).Error; err != nil {
		t.Fatal(err)
	}
	applicationStore := newApplicationPlatform(ds)
	if _, err := applicationStore.PublishApplicationVersion(t.Context(), versionID); err != nil {
		t.Fatal(err)
	}
	assertWatermillPayloadCount(t, db, "watermill_application_version_published", versionID, 1)

	task := &iapiserver.AtomicTask{
		FunctionRef:     "application-platform.run",
		Arguments:       map[string]any{"application_version_id": versionID},
		RetryPolicy:     iapiserver.RetryPolicy{},
		TimeoutPolicy:   iapiserver.TimeoutPolicy{},
		CancelPolicy:    map[string]any{},
		Status:          iapiserver.AtomicTaskStatusSuccess,
		Progress:        1,
		Output:          map[string]any{},
		LastError:       iapiserver.TaskError{},
		OwnerType:       iapiserver.TaskOwnerTypeDAGGroup,
		OwnerID:         "dag-" + suffix,
		ChildKey:        "application",
		DAGNodeKey:      "application",
		CanvasRunID:     canvasRunID,
		CanvasNodeRunID: nodeRunID,
		ProjectID:       iapiserver.DefaultTaskCenterProjectID,
		Namespace:       iapiserver.DefaultTaskCenterNamespace,
		CreatedBy:       application.OwnerUserID,
	}
	task.ID = taskID
	task.Name = "Application task"
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	run := &iapiserver.WorkflowCanvasRun{
		CanvasID:            "canvas-" + suffix,
		CanvasVersionID:     "canvas-version-" + suffix,
		IdempotencyKey:      "canvas-key-" + suffix,
		RequestDigest:       "sha256:" + suffix,
		InputSnapshot:       map[string]any{},
		Scope:               iapiserver.WorkflowRunScope{Mode: iapiserver.CanvasRunScopeAll},
		RunPolicy:           iapiserver.WorkflowRunPolicy{},
		ReuseDecisions:      []map[string]any{},
		ExecutionPlan:       map[string]any{},
		ExecutionPlanDigest: "sha256:" + suffix,
		TaskCreationStatus:  iapiserver.CanvasTaskCreationCreated,
		Status:              iapiserver.CanvasRunStatusRunning,
		Summary:             map[string]any{},
		ResultSummary:       map[string]any{},
		Warnings:            []iapiserver.WorkflowRunWarning{},
		LastError:           map[string]any{},
		AggregateVersion:    1,
		ProjectID:           iapiserver.DefaultTaskCenterProjectID,
		Namespace:           iapiserver.DefaultTaskCenterNamespace,
		CreatedBy:           application.OwnerUserID,
	}
	run.ID = canvasRunID
	run.Name = "Canvas run"
	if err := db.Create(run).Error; err != nil {
		t.Fatal(err)
	}
	node := &iapiserver.CanvasNodeRun{
		CanvasRunID:              canvasRunID,
		NodeID:                   "application",
		ExecutionKey:             "application",
		NodeType:                 iapiserver.CanvasNodeTypeApplication,
		DefinitionVersion:        version.SemanticVersion,
		ExecutionFingerprint:     "sha256:" + suffix,
		ResolvedInputSnapshot:    map[string]any{},
		ResultMode:               iapiserver.CanvasResultExecuted,
		Status:                   iapiserver.AtomicTaskStatusRunning,
		Progress:                 1,
		TaskCount:                1,
		OutputCount:              1,
		RequiredOutputCount:      1,
		ReadyRequiredOutputCount: 0,
		Warnings:                 []iapiserver.WorkflowRunWarning{},
		LastError:                map[string]any{},
		AggregateVersion:         1,
	}
	node.ID = nodeRunID
	node.Name = "application"
	if err := db.Create(node).Error; err != nil {
		t.Fatal(err)
	}
	taskBinding := &iapiserver.CanvasNodeRunTaskBinding{
		CanvasNodeRunID:     nodeRunID,
		DAGTaskGroupID:      task.OwnerID,
		AtomicTaskID:        taskID,
		TaskChildKey:        task.ChildKey,
		BindingRole:         "primary",
		ShardKey:            "root",
		TaskResourceVersion: task.ResourceVersion,
	}
	taskBinding.ID = "task-binding-" + suffix
	taskBinding.Name = task.ChildKey
	if err := db.Create(taskBinding).Error; err != nil {
		t.Fatal(err)
	}
	outputBinding := &iapiserver.CanvasNodeRunOutputBinding{
		CanvasNodeRunID:         nodeRunID,
		PortKey:                 "image",
		Required:                true,
		ShardKey:                "root",
		ProducerKey:             canvasRunID + ":" + nodeRunID + ":image:root",
		AvailabilityStatus:      "PENDING",
		ArtifactResourceVersion: 0,
		AggregateVersion:        1,
	}
	outputBinding.ID = "output-binding-" + suffix
	outputBinding.Name = "image"
	if err := db.Create(outputBinding).Error; err != nil {
		t.Fatal(err)
	}

	canvasStore := newWorkflowCanvasStore(ds)
	applied, err := canvasStore.ProjectCanvasApplicationArtifact(t.Context(), &store.CanvasApplicationArtifactProjection{
		AtomicTaskID:             taskID,
		OutputKey:                "image",
		Sequence:                 0,
		ArtifactID:               artifactID,
		MediaType:                "image",
		ArtifactProcessingStatus: iapiserver.ArtifactProcessingReady,
		ArtifactResourceVersion:  2,
	})
	if err != nil || !applied {
		t.Fatalf("ready projection applied=%t error=%v", applied, err)
	}
	applied, err = canvasStore.ProjectCanvasApplicationArtifact(t.Context(), &store.CanvasApplicationArtifactProjection{
		AtomicTaskID:             taskID,
		OutputKey:                "image",
		Sequence:                 0,
		ArtifactID:               artifactID,
		MediaType:                "image",
		ArtifactProcessingStatus: iapiserver.ArtifactProcessingFailed,
		ArtifactResourceVersion:  1,
	})
	if err != nil || applied {
		t.Fatalf("stale projection applied=%t error=%v", applied, err)
	}
	var projectedBinding iapiserver.CanvasNodeRunOutputBinding
	if err := db.Where("id = ?", outputBinding.ID).First(&projectedBinding).Error; err != nil {
		t.Fatal(err)
	}
	var projectedNode iapiserver.CanvasNodeRun
	if err := db.Where("id = ?", nodeRunID).First(&projectedNode).Error; err != nil {
		t.Fatal(err)
	}
	if projectedBinding.AvailabilityStatus != "READY" ||
		projectedBinding.ArtifactResourceVersion != 2 ||
		projectedNode.Status != iapiserver.AtomicTaskStatusSuccess ||
		projectedNode.ReadyRequiredOutputCount != 1 {
		t.Fatalf("binding=%#v node=%#v", projectedBinding, projectedNode)
	}
	assertWatermillPayloadCount(t, db, "watermill_canvas_node_output_available", artifactID, 1)

	applicationRun := &iapiserver.ApplicationRun{
		OwnerUserID:                  application.OwnerUserID,
		ApplicationID:                applicationID,
		ApplicationVersionID:         versionID,
		ApplicationTemplateVersionID: version.ApplicationTemplateVersionID,
		EngineInstanceID:             "engine-" + suffix,
		CapabilitySourceType:         iapiserver.CapabilitySourceProviderCapability,
		SourceRevision:               "revision-" + suffix,
		CapabilitySourceSnapshot:     map[string]any{},
		InputSnapshot:                map[string]any{},
		ExecutionSnapshot: map[string]any{
			"origin_type":        "canvas",
			"canvas_run_id":      canvasRunID,
			"canvas_node_run_id": nodeRunID,
			"execution_key":      "application",
		},
		OutputMappingSnapshot: map[string]any{},
		TaskCreationStatus:    iapiserver.TaskCreationPending,
		OutputValues:          []map[string]any{},
		IdempotencyKey:        "canvas:" + canvasRunID + ":application",
	}
	applicationRun.ID = applicationRunID
	applicationRun.Name = "Application run"
	if _, err := applicationStore.AddApplicationRun(t.Context(), applicationRun); err != nil {
		t.Fatal(err)
	}
	taskStore := newTaskCenterStore(ds)
	finalArguments := map[string]any{
		"application_version_id": versionID,
		"resolved_inputs":        map[string]any{"prompt": "hello"},
	}
	boundTask, err := taskStore.BindApplicationRunToAtomicTask(
		t.Context(),
		taskID,
		applicationRunID,
		canvasRunID,
		nodeRunID,
		task.ChildKey,
		finalArguments,
	)
	if err != nil {
		t.Fatal(err)
	}
	boundVersion := boundTask.ResourceVersion
	boundTask, err = taskStore.BindApplicationRunToAtomicTask(
		t.Context(),
		taskID,
		applicationRunID,
		canvasRunID,
		nodeRunID,
		task.ChildKey,
		finalArguments,
	)
	if err != nil {
		t.Fatal(err)
	}
	if boundTask.ResourceVersion != boundVersion {
		t.Fatalf("idempotent task bind resource version = %d, want %d", boundTask.ResourceVersion, boundVersion)
	}
	if _, err := applicationStore.BindApplicationRunTask(
		t.Context(),
		applicationRunID,
		taskID,
		iapiserver.TaskCreationCreated,
		iapiserver.AtomicTaskStatusSuccess,
		boundTask.ResourceVersion,
		"",
	); err != nil {
		t.Fatal(err)
	}
	assertWatermillPayloadCount(t, db, "watermill_application_run_created", applicationRunID, 1)
	assertWatermillPayloadCount(t, db, "watermill_application_run_atomic_task_bound", applicationRunID, 1)
}

func assertWatermillPayloadCount(t *testing.T, db *gorm.DB, table, marker string, want int64) {
	t.Helper()
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE payload::text LIKE ?", table)
	if err := db.Raw(query, "%"+marker+"%").Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("%s payload count for %q = %d, want %d", table, marker, count, want)
	}
}

func cleanupCanvasApplicationIntegrationRows(
	t *testing.T,
	db *gorm.DB,
	suffix, applicationID, versionID, applicationRunID, canvasRunID, nodeRunID, taskID string,
) {
	t.Helper()
	for _, table := range []string{
		"watermill_application_version_published",
		"watermill_application_run_created",
		"watermill_application_run_atomic_task_bound",
		"watermill_canvas_node_output_available",
		"watermill_canvas_node_run_status_changed",
		"watermill_canvas_run_status_changed",
	} {
		if err := db.Exec(fmt.Sprintf("DELETE FROM %s WHERE payload::text LIKE ?", table), "%"+suffix+"%").Error; err != nil {
			t.Logf("cleanup %s: %v", table, err)
		}
	}
	for _, item := range []struct {
		model any
		id    string
	}{
		{&iapiserver.WorkflowCanvasOutbox{}, ""},
		{&iapiserver.CanvasNodeRunOutputBinding{}, ""},
		{&iapiserver.CanvasNodeRunTaskBinding{}, ""},
		{&iapiserver.CanvasNodeRun{}, nodeRunID},
		{&iapiserver.WorkflowCanvasRun{}, canvasRunID},
		{&iapiserver.AtomicTask{}, taskID},
		{&iapiserver.ApplicationRun{}, applicationRunID},
		{&iapiserver.ApplicationVersion{}, versionID},
		{&iapiserver.Application{}, applicationID},
	} {
		query := db
		if item.id == "" {
			query = query.Where("name LIKE ?", "%"+suffix+"%")
		} else {
			query = query.Where("id = ?", item.id)
		}
		if err := query.Delete(item.model).Error; err != nil {
			t.Logf("cleanup %T: %v", item.model, err)
		}
	}
}
