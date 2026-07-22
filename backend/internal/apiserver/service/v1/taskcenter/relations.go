package taskcenter

import (
	"context"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

// attachAtomicTaskRelations 批量补充任务链和多态 owner 摘要，查询次数只随关系类型增长。
func (s *taskCenterService) attachAtomicTaskRelations(ctx context.Context, tasks []*iapiserver.AtomicTask) error {
	if len(tasks) == 0 {
		return nil
	}

	taskByID := make(map[string]*iapiserver.AtomicTask, len(tasks))
	relatedTaskIDs := make(map[string]struct{})
	ownerIDs := map[string]map[string]struct{}{
		iapiserver.TaskOwnerTypeGroup:    {},
		iapiserver.TaskOwnerTypeDAGGroup: {},
		iapiserver.TaskOwnerTypeSchedule: {},
	}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		taskByID[task.ID] = task
		if task.RetryOfTaskID != "" {
			relatedTaskIDs[task.RetryOfTaskID] = struct{}{}
		}
		if task.RootTaskID != "" {
			relatedTaskIDs[task.RootTaskID] = struct{}{}
		}
		if ids, ok := ownerIDs[task.OwnerType]; ok && task.OwnerID != "" {
			ids[task.OwnerID] = struct{}{}
		}
	}

	missingTaskIDs := make([]string, 0, len(relatedTaskIDs))
	for id := range relatedTaskIDs {
		if _, exists := taskByID[id]; !exists {
			missingTaskIDs = append(missingTaskIDs, id)
		}
	}
	if len(missingTaskIDs) > 0 {
		related, err := s.store.GetAtomicTasksByIDs(ctx, missingTaskIDs)
		if err != nil {
			return err
		}
		for _, task := range related {
			taskByID[task.ID] = task
		}
	}

	groupByID, err := s.taskGroupOwners(ctx, ownerIDs[iapiserver.TaskOwnerTypeGroup])
	if err != nil {
		return err
	}
	dagByID, err := s.dagTaskGroupOwners(ctx, ownerIDs[iapiserver.TaskOwnerTypeDAGGroup])
	if err != nil {
		return err
	}
	scheduleByID, err := s.taskScheduleOwners(ctx, ownerIDs[iapiserver.TaskOwnerTypeSchedule])
	if err != nil {
		return err
	}

	for _, task := range tasks {
		if task == nil {
			continue
		}
		if related := taskByID[task.RetryOfTaskID]; visibleRelatedTask(ctx, task, related) {
			task.RetryOfTask = atomicTaskSummary(related)
		}
		if related := taskByID[task.RootTaskID]; visibleRelatedTask(ctx, task, related) {
			task.RootTask = atomicTaskSummary(related)
		}
		switch task.OwnerType {
		case iapiserver.TaskOwnerTypeGroup:
			if owner := groupByID[task.OwnerID]; visibleTaskGroupOwner(ctx, task, owner) {
				task.Owner = taskGroupSummary(owner)
			}
		case iapiserver.TaskOwnerTypeDAGGroup:
			if owner := dagByID[task.OwnerID]; visibleDAGTaskGroupOwner(ctx, task, owner) {
				task.Owner = dagTaskGroupSummary(owner)
			}
		case iapiserver.TaskOwnerTypeSchedule:
			if owner := scheduleByID[task.OwnerID]; visibleTaskScheduleOwner(ctx, task, owner) {
				task.Owner = taskScheduleOwnerSummary(owner)
			}
		}
	}
	return s.attachTaskArtifactSummaries(ctx, tasks)
}

// attachTaskArtifactSummaries 按最多 200 项批量调用 asset-library，并在原始 artifact_id 旁附加可见摘要。
func (s *taskCenterService) attachTaskArtifactSummaries(ctx context.Context, tasks []*iapiserver.AtomicTask) error {
	if s.artifacts == nil {
		return nil
	}
	ids := make([]string, 0)
	seen := make(map[string]bool)
	for _, task := range tasks {
		for _, ref := range taskArtifactRefs(task) {
			id, _ := ref["artifact_id"].(string)
			if id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	summaries := make(map[string]*iapiserver.ArtifactReadableSummary, len(ids))
	for start := 0; start < len(ids); start += 200 {
		end := min(start+200, len(ids))
		batch, err := s.artifacts.ResolveArtifactSummaries(ctx, taskActor(ctx), ids[start:end])
		if err != nil {
			return err
		}
		for id, summary := range batch {
			summaries[id] = summary
		}
	}
	for _, task := range tasks {
		for _, ref := range taskArtifactRefs(task) {
			id, _ := ref["artifact_id"].(string)
			ref["artifact"] = summaries[id]
		}
	}
	return nil
}

func taskArtifactRefs(task *iapiserver.AtomicTask) []map[string]any {
	if task == nil || task.Output == nil {
		return nil
	}
	value := task.Output["artifact_refs"]
	switch refs := value.(type) {
	case []map[string]any:
		return refs
	case []any:
		result := make([]map[string]any, 0, len(refs))
		for _, item := range refs {
			if ref, ok := item.(map[string]any); ok {
				result = append(result, ref)
			}
		}
		return result
	default:
		return nil
	}
}

func (s *taskCenterService) attachTaskGroupRelations(ctx context.Context, groups []*iapiserver.TaskGroup) error {
	ids := make(map[string]struct{})
	for _, group := range groups {
		if group != nil && group.RetryOfID != "" {
			ids[group.RetryOfID] = struct{}{}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	related, err := s.store.GetTaskGroupsByIDs(ctx, setIDs(ids))
	if err != nil {
		return err
	}
	byID := make(map[string]*iapiserver.TaskGroup, len(related))
	for _, group := range related {
		byID[group.ID] = group
	}
	for _, group := range groups {
		if source := byID[group.RetryOfID]; visibleTaskGroupRetry(ctx, group, source) {
			group.RetryOf = taskGroupSummary(source)
		}
	}
	return nil
}

func (s *taskCenterService) attachDAGTaskGroupRelations(ctx context.Context, groups []*iapiserver.DAGTaskGroup) error {
	ids := make(map[string]struct{})
	for _, group := range groups {
		if group != nil && group.RetryOfID != "" {
			ids[group.RetryOfID] = struct{}{}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	related, err := s.store.GetDAGTaskGroupsByIDs(ctx, setIDs(ids))
	if err != nil {
		return err
	}
	byID := make(map[string]*iapiserver.DAGTaskGroup, len(related))
	for _, group := range related {
		byID[group.ID] = group
	}
	for _, group := range groups {
		if source := byID[group.RetryOfID]; visibleDAGTaskGroupRetry(ctx, group, source) {
			group.RetryOf = dagTaskGroupSummary(source)
		}
	}
	return nil
}

func (s *taskCenterService) taskGroupOwners(ctx context.Context, ids map[string]struct{}) (map[string]*iapiserver.TaskGroup, error) {
	if len(ids) == 0 {
		return map[string]*iapiserver.TaskGroup{}, nil
	}
	items, err := s.store.GetTaskGroupsByIDs(ctx, setIDs(ids))
	if err != nil {
		return nil, err
	}
	result := make(map[string]*iapiserver.TaskGroup, len(items))
	for _, item := range items {
		result[item.ID] = item
	}
	return result, nil
}

func (s *taskCenterService) dagTaskGroupOwners(ctx context.Context, ids map[string]struct{}) (map[string]*iapiserver.DAGTaskGroup, error) {
	if len(ids) == 0 {
		return map[string]*iapiserver.DAGTaskGroup{}, nil
	}
	items, err := s.store.GetDAGTaskGroupsByIDs(ctx, setIDs(ids))
	if err != nil {
		return nil, err
	}
	result := make(map[string]*iapiserver.DAGTaskGroup, len(items))
	for _, item := range items {
		result[item.ID] = item
	}
	return result, nil
}

func (s *taskCenterService) taskScheduleOwners(ctx context.Context, ids map[string]struct{}) (map[string]*iapiserver.TaskSchedule, error) {
	if len(ids) == 0 {
		return map[string]*iapiserver.TaskSchedule{}, nil
	}
	items, err := s.store.GetTaskSchedulesByIDs(ctx, setIDs(ids))
	if err != nil {
		return nil, err
	}
	result := make(map[string]*iapiserver.TaskSchedule, len(items))
	for _, item := range items {
		result[item.ID] = item
	}
	return result, nil
}

func atomicTaskSummary(task *iapiserver.AtomicTask) *iapiserver.AtomicTaskSummary {
	if task == nil {
		return nil
	}
	return &iapiserver.AtomicTaskSummary{ID: task.ID, Name: task.Name, Status: task.Status, Progress: task.Progress, FunctionRef: task.FunctionRef}
}

func taskGroupSummary(group *iapiserver.TaskGroup) *iapiserver.TaskOwnerSummary {
	if group == nil {
		return nil
	}
	return &iapiserver.TaskOwnerSummary{Type: iapiserver.TaskOwnerTypeGroup, ID: group.ID, Name: group.Name, Status: group.Status, Progress: group.Progress}
}

func dagTaskGroupSummary(group *iapiserver.DAGTaskGroup) *iapiserver.TaskOwnerSummary {
	if group == nil {
		return nil
	}
	return &iapiserver.TaskOwnerSummary{Type: iapiserver.TaskOwnerTypeDAGGroup, ID: group.ID, Name: group.Name, Status: group.Status, Progress: group.Progress}
}

func taskScheduleOwnerSummary(schedule *iapiserver.TaskSchedule) *iapiserver.TaskOwnerSummary {
	if schedule == nil {
		return nil
	}
	return &iapiserver.TaskOwnerSummary{Type: iapiserver.TaskOwnerTypeSchedule, ID: schedule.ID, Name: schedule.Name, Status: schedule.Status}
}

func taskScheduleSummary(schedule *iapiserver.TaskSchedule) *iapiserver.TaskScheduleSummary {
	if schedule == nil {
		return nil
	}
	return &iapiserver.TaskScheduleSummary{ID: schedule.ID, Name: schedule.Name, Status: schedule.Status}
}

func visibleRelatedTask(ctx context.Context, parent, related *iapiserver.AtomicTask) bool {
	return related != nil && parent.ProjectID == related.ProjectID && parent.Namespace == related.Namespace && canReadTaskCreatedBy(ctx, related.CreatedBy)
}

func visibleTaskGroupOwner(ctx context.Context, task *iapiserver.AtomicTask, owner *iapiserver.TaskGroup) bool {
	return owner != nil && task.ProjectID == owner.ProjectID && task.Namespace == owner.Namespace && canReadTaskCreatedBy(ctx, owner.CreatedBy)
}

func visibleDAGTaskGroupOwner(ctx context.Context, task *iapiserver.AtomicTask, owner *iapiserver.DAGTaskGroup) bool {
	return owner != nil && task.ProjectID == owner.ProjectID && task.Namespace == owner.Namespace && canReadTaskCreatedBy(ctx, owner.CreatedBy)
}

func visibleTaskScheduleOwner(ctx context.Context, task *iapiserver.AtomicTask, owner *iapiserver.TaskSchedule) bool {
	return owner != nil && task.ProjectID == owner.ProjectID && task.Namespace == owner.Namespace && canReadTaskSchedule(ctx, owner)
}

func visibleTaskGroupRetry(ctx context.Context, parent, source *iapiserver.TaskGroup) bool {
	return source != nil && parent.ProjectID == source.ProjectID && parent.Namespace == source.Namespace && canReadTaskCreatedBy(ctx, source.CreatedBy)
}

func visibleDAGTaskGroupRetry(ctx context.Context, parent, source *iapiserver.DAGTaskGroup) bool {
	return source != nil && parent.ProjectID == source.ProjectID && parent.Namespace == source.Namespace && canReadTaskCreatedBy(ctx, source.CreatedBy)
}

func setIDs(values map[string]struct{}) []string {
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	return ids
}
