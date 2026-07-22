//go:build integration

package postgresql

import (
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestWorkflowCanvasV17MigrationBackfillsLegacyBindings(t *testing.T) {
	dsn := os.Getenv("WORKFLOW_CANVAS_TEST_DSN")
	if dsn == "" {
		t.Skip("WORKFLOW_CANVAS_TEST_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	legacy := []string{
		`CREATE TABLE canvases (id text primary key,name text not null,created_at timestamptz not null,updated_at timestamptz not null,description text default '',extend_shadow text default '',resource_version integer default 1,visibility text not null,draft_graph_json text not null,draft_revision bigint not null,latest_version integer not null,project_id text not null,namespace text not null,created_by text not null,deleted_at timestamptz)`,
		`CREATE TABLE canvas_versions (id text primary key,name text not null,created_at timestamptz not null,updated_at timestamptz not null,description text default '',extend_shadow text default '',resource_version integer default 1,canvas_id text not null,version integer not null,graph_snapshot_json text not null,input_schema_json text not null,output_schema_json text not null,content_digest text not null,compiled_definition_name text not null,compiled_definition_version integer not null,node_count integer not null,edge_count integer not null,published_by text not null,published_at timestamptz not null)`,
		`CREATE TABLE canvas_runs (id text primary key,name text not null,created_at timestamptz not null,updated_at timestamptz not null,description text default '',extend_shadow text default '',resource_version integer default 1,canvas_id text not null,canvas_version_id text not null,idempotency_key text not null,request_digest text not null,input_snapshot_json text not null,dag_task_group_id text,task_creation_status text not null,status text not null,progress real not null,summary_json text not null,output_json text not null,last_error_json text not null,task_resource_version bigint not null,retry_of_canvas_run_id text,project_id text not null,namespace text not null,created_by text not null,finished_at timestamptz)`,
		`CREATE TABLE canvas_node_runs (id text primary key,name text not null,created_at timestamptz not null,updated_at timestamptz not null,description text default '',extend_shadow text default '',resource_version integer default 1,canvas_run_id text not null,node_key text not null,node_type text not null,atomic_task_id text,status text not null,progress real not null,output_json text not null,last_error_json text not null,task_resource_version bigint not null,finished_at timestamptz)`,
		`INSERT INTO canvases VALUES ('canvas-1','Legacy',now(),now(),'','',1,'PRIVATE','{"nodes":[],"edges":[]}',1,1,'project','default','user-1',NULL)`,
		`INSERT INTO canvas_versions VALUES ('version-1','Legacy v1',now(),now(),'','',1,'canvas-1',1,'{"nodes":[],"edges":[]}','{}','{}','sha256:legacy','legacy_definition',1,1,0,'user-1',now())`,
		`INSERT INTO canvas_runs VALUES ('run-1','Run',now(),now(),'','',1,'canvas-1','version-1','key','sha256:request','{}','dag-1','CREATED','RUNNING',0,'{}','{}','{}',1,NULL,'project','default','user-1',NULL)`,
		`INSERT INTO canvas_node_runs VALUES ('node-run-1','node_a',now(),now(),'','',1,'run-1','node_a','FUNCTION','task-1','RUNNING',0,'{}','{}',3,NULL)`,
	}
	for _, statement := range legacy {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	models := []any{
		&iapiserver.WorkflowNodeDefinition{},
		&iapiserver.WorkflowCanvas{},
		&iapiserver.CanvasVersion{},
		&iapiserver.WorkflowCanvasRun{},
		&iapiserver.CanvasFlowRun{},
		&iapiserver.CanvasNodeRun{},
		&iapiserver.CanvasNodeRunFlowRef{},
		&iapiserver.CanvasNodeRunTaskBinding{},
		&iapiserver.CanvasNodeRunOutputBinding{},
		&iapiserver.WorkflowCanvasOutbox{},
		&iapiserver.WorkflowCanvasReconcileCursor{},
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	ds := &datastore{db: db}
	if err := ds.ensureWorkflowCanvasScheme(); err != nil {
		t.Fatal(err)
	}
	var node iapiserver.CanvasNodeRun
	if err := db.Where("id = ?", "node-run-1").First(&node).Error; err != nil {
		t.Fatal(err)
	}
	if node.NodeID != "node_a" || node.ExecutionKey != "node_a" || node.ExecutionFingerprint == "" {
		t.Fatalf("node=%#v", node)
	}
	var binding iapiserver.CanvasNodeRunTaskBinding
	if err := db.Where("atomic_task_id = ?", "task-1").First(&binding).Error; err != nil {
		t.Fatal(err)
	}
	if binding.CanvasNodeRunID != "node-run-1" || binding.TaskResourceVersion != 3 {
		t.Fatalf("binding=%#v", binding)
	}
}
