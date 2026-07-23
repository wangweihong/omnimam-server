package taskcenter

import (
	"maps"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/taskname"
)

func assignSystemName(name *string, meta *iapiserver.TaskNameMeta, spec iapiserver.SystemNameSpec) error {
	if spec.Key == "" {
		meta.NameSource = iapiserver.TaskNameSourceUser
		meta.SystemNameKey = ""
		meta.SystemNameParams = map[string]string{}
		meta.NameI18n = nil
		return nil
	}
	localized, err := taskname.Resolve(spec.Key, spec.Params)
	if err != nil {
		return err
	}
	meta.NameSource = iapiserver.TaskNameSourceSystem
	meta.SystemNameKey = spec.Key
	meta.SystemNameParams = maps.Clone(spec.Params)
	meta.NameI18n = localized
	*name = localized[taskname.LanguageEnglish]
	return nil
}

func projectLocalizedName(meta *iapiserver.TaskNameMeta) {
	meta.NameI18n = nil
	if meta.NameSource != iapiserver.TaskNameSourceSystem || meta.SystemNameKey == "" {
		return
	}
	localized, err := taskname.Resolve(meta.SystemNameKey, meta.SystemNameParams)
	if err == nil {
		meta.NameI18n = localized
	}
}

func localizedName(meta iapiserver.TaskNameMeta) map[string]string {
	projectLocalizedName(&meta)
	return maps.Clone(meta.NameI18n)
}

func systemNameSpec(meta iapiserver.TaskNameMeta) iapiserver.SystemNameSpec {
	if meta.NameSource != iapiserver.TaskNameSourceSystem {
		return iapiserver.SystemNameSpec{}
	}
	return iapiserver.SystemNameSpec{Key: meta.SystemNameKey, Params: maps.Clone(meta.SystemNameParams)}
}

func localizeAtomicTasks(tasks []*iapiserver.AtomicTask) {
	for _, task := range tasks {
		if task != nil {
			projectLocalizedName(&task.TaskNameMeta)
		}
	}
}

func localizeTaskGroups(groups []*iapiserver.TaskGroup) {
	for _, group := range groups {
		if group != nil {
			projectLocalizedName(&group.TaskNameMeta)
		}
	}
}

func localizeDAGTaskGroups(groups []*iapiserver.DAGTaskGroup) {
	for _, group := range groups {
		if group != nil {
			projectLocalizedName(&group.TaskNameMeta)
		}
	}
}

func localizeTaskSchedules(schedules []*iapiserver.TaskSchedule) {
	for _, schedule := range schedules {
		if schedule != nil {
			projectLocalizedName(&schedule.TaskNameMeta)
		}
	}
}
