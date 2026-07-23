package taskcenter

import (
	"reflect"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/taskname"
)

func TestAssignSystemName(t *testing.T) {
	name := "legacy fallback"
	meta := iapiserver.TaskNameMeta{}
	err := assignSystemName(&name, &meta, iapiserver.SystemNameSpec{
		Key:    taskname.RepresentationGenerate,
		Params: map[string]string{"representation_type": "thumbnail"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "Generate thumbnail" || meta.NameSource != iapiserver.TaskNameSourceSystem || meta.SystemNameKey != taskname.RepresentationGenerate {
		t.Fatalf("system name was not assigned: name=%q meta=%#v", name, meta)
	}
	want := map[string]string{"zh-CN": "生成 thumbnail 表现形式", "en-US": "Generate thumbnail"}
	if !reflect.DeepEqual(meta.NameI18n, want) {
		t.Fatalf("name_i18n = %#v, want %#v", meta.NameI18n, want)
	}

	meta.SystemNameParams["representation_type"] = "changed"
	if got := meta.NameI18n[taskname.LanguageEnglish]; got != "Generate thumbnail" {
		t.Fatalf("resolved names alias parameters: %q", got)
	}
}

func TestAssignUserNameDoesNotCreateLocalizedProjection(t *testing.T) {
	name := "User supplied name"
	meta := iapiserver.TaskNameMeta{
		NameSource:       iapiserver.TaskNameSourceSystem,
		SystemNameKey:    taskname.ApplicationRun,
		SystemNameParams: map[string]string{},
		NameI18n:         map[string]string{"en-US": "stale"},
	}
	if err := assignSystemName(&name, &meta, iapiserver.SystemNameSpec{}); err != nil {
		t.Fatal(err)
	}
	if name != "User supplied name" || meta.NameSource != iapiserver.TaskNameSourceUser || meta.SystemNameKey != "" || meta.NameI18n != nil {
		t.Fatalf("user name metadata was not cleared: name=%q meta=%#v", name, meta)
	}
}

func TestLocalizedSummaryUsesCatalogAndIgnoresLegacyRows(t *testing.T) {
	system := &iapiserver.AtomicTask{}
	system.Name = "Generate asset thumbnail"
	system.TaskNameMeta = iapiserver.TaskNameMeta{NameSource: iapiserver.TaskNameSourceSystem, SystemNameKey: taskname.AssetThumbnail, SystemNameParams: map[string]string{}}
	if got := atomicTaskSummary(system).NameI18n; got[taskname.LanguageChinese] != "生成素材缩略图" {
		t.Fatalf("system summary name_i18n = %#v", got)
	}

	legacy := &iapiserver.AtomicTask{}
	legacy.Name = "Generate asset thumbnail"
	legacy.TaskNameMeta = iapiserver.TaskNameMeta{NameSource: iapiserver.TaskNameSourceUser}
	if got := atomicTaskSummary(legacy).NameI18n; got != nil {
		t.Fatalf("legacy user row was heuristically translated: %#v", got)
	}
}

func TestTasksFromTemplatesPreservesSystemNameMetadata(t *testing.T) {
	tasks, err := tasksFromTemplates([]iapiserver.AtomicTaskTemplate{{
		Key:        "thumbnail",
		Name:       "fallback",
		SystemName: iapiserver.SystemNameSpec{Key: taskname.RepresentationGenerate, Params: map[string]string{"representation_type": "thumbnail"}},
	}}, "group-1", iapiserver.TaskOwnerTypeGroup, "project", "namespace", "system")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Name != "Generate thumbnail" || tasks[0].SystemNameKey != taskname.RepresentationGenerate {
		t.Fatalf("system child task = %#v", tasks)
	}
}
