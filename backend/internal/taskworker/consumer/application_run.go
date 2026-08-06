package consumer

import (
	"context"
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/postgresql"
)

type applicationRunTerminalProjector interface {
	Completed(context.Context, *iapiserver.AtomicTask) error
}

// StartApplicationRunTerminalProjection 使用持久 offset 启动 ApplicationRun 终态投影消费。
func StartApplicationRunTerminalProjection(
	ctx context.Context,
	tasks store.TaskCenterStore,
	projector applicationRunTerminalProjector,
) error {
	messages, err := postgresql.SubscribeOutbox(
		ctx,
		postgresql.OutboxTopicAtomicTaskStatusChanged,
		iapiserver.TaskWorkerConsumerGroupApplicationRunTerminal,
	)
	if err != nil {
		return err
	}
	go ConsumeApplicationRunTerminalProjections(ctx, messages, tasks, projector)
	return nil
}

// ConsumeApplicationRunTerminalProjections 投影终态任务，并按处理结果确认或拒绝消息。
func ConsumeApplicationRunTerminalProjections(
	ctx context.Context,
	messages <-chan *message.Message,
	tasks store.TaskCenterStore,
	projector applicationRunTerminalProjector,
) {
	for msg := range messages {
		if err := HandleApplicationRunTerminalProjection(ctx, tasks, projector, msg.Payload); err != nil {
			log.Errorf(
				"application run terminal projection failed: consumer_group=%s message_id=%s error=%v",
				iapiserver.TaskWorkerConsumerGroupApplicationRunTerminal,
				msg.UUID,
				err,
			)
			msg.Nack()
		} else {
			msg.Ack()
		}
	}
}

// HandleApplicationRunTerminalProjection 校验状态事件并投影当前终态 AtomicTask。
func HandleApplicationRunTerminalProjection(
	ctx context.Context,
	tasks store.TaskCenterStore,
	projector applicationRunTerminalProjector,
	payload []byte,
) error {
	var event struct {
		AtomicTaskID     string `json:"atomic_task_id"`
		ApplicationRunID string `json:"application_run_id"`
		Status           string `json:"status"`
		ToStatus         string `json:"to_status"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "decode atomic task status event")
	}
	if event.AtomicTaskID == "" {
		return errors.Errorf("atomic task status event is incomplete")
	}
	status := event.ToStatus
	if status == "" {
		status = event.Status
	}
	if event.ApplicationRunID == "" || !iapiserver.IsAtomicTaskTerminal(status) {
		return nil
	}
	task, err := tasks.GetAtomicTask(ctx, event.AtomicTaskID)
	if err != nil {
		return errors.Wrap(err, "load terminal application run atomic task")
	}
	if task == nil {
		return errors.Errorf("terminal application run atomic task is missing")
	}
	if task.ApplicationRunID == "" || !iapiserver.IsAtomicTaskTerminal(task.Status) {
		return nil
	}
	return errors.Wrap(projector.Completed(ctx, task), "project terminal application run")
}
