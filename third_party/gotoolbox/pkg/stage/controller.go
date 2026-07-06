package stage

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/typeutil"
)

const (
	// 初始化中
	StateInitializing = "initializing"
	// 运行中
	StateRunning = "running"
	// 主动停止
	StateStop = "stop"
	// 遇到问题
	StateError = "error"
	// 完成
	StateComplete = "complete"
)

func NewExecuteStageController(ctx context.Context, name string, stages ...Stage) *ExecuteStageController {
	return &ExecuteStageController{
		Name:   name,
		stages: stages,
		state:  StateInitializing,
	}
}

func NewAsyncExecuteStageController(ctx context.Context, name string, stages ...Stage) *ExecuteStageController {
	return &ExecuteStageController{
		Name:   name,
		stages: stages,
		state:  StateInitializing,
		async:  true,
	}
}

func BuildControllerFromMeta(m *ControllerMeta) *ExecuteStageController {
	return &ExecuteStageController{
		Name:         m.Name,
		stages:       m.Stages,
		state:        m.State,
		async:        m.Async,
		currentStage: m.CurrentStage,
		isStop:       m.IsStop,
	}
}

type ControllerMeta struct {
	Name         string
	State        string // 当前状态
	CurrentStage int
	Stages       []Stage
	IsStop       bool // stop state running
	Async        bool // 是否异步执行
}

type ExecuteStageController struct {
	ctx              context.Context
	Name             string
	lock             sync.Mutex
	stages           []Stage
	currentStage     int
	state            string                          // 当前状态
	stageSaveFun     func(ctx context.Context) error // 备份函数
	stagesFinishFunc func(ctx context.Context) error // 所有阶段执行结束后处理函数
	isStop           bool                            // stop state running
	async            bool                            // 是否异步执行
}

func (d *ExecuteStageController) GetMeta() *ControllerMeta {
	d.lock.Lock()
	defer d.lock.Unlock()

	return &ControllerMeta{
		Name:         d.Name,
		State:        d.state,
		CurrentStage: d.currentStage,
		Stages:       d.stages,
		IsStop:       d.isStop,
		Async:        d.async,
	}
}

func (d *ExecuteStageController) GetError() error {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.state != StateError {
		return nil
	}

	//bug?
	if d.currentStage >= len(d.stages) {
		return nil
	}

	currentStage := d.stages[d.currentStage]
	return fmt.Errorf("stage %v meet error:%v", currentStage.Name, currentStage.ErrorMessage)
}

func (d *ExecuteStageController) Run() error {
	//only initializing, stop state can rerun.
	updateState := func() error {
		d.lock.Lock()
		defer d.lock.Unlock()
		switch d.state {
		case StateRunning:
			return fmt.Errorf("state controller has run")
		case StateError:
			return fmt.Errorf("state controller run fail")
		case StateComplete:
			return fmt.Errorf("state controller run complete")
		}

		d.state = StateRunning
		d.isStop = false
		return nil
	}
	if err := updateState(); err != nil {
		return err
	}

	if d.async {
		go func() {
			defer func() {
				if x := recover(); x != nil {
					fmt.Println("run time panic: ", x, string(debug.Stack()))
				}
				d.lock.Lock()
				d.state = StateError
				d.lock.Unlock()
			}()
			d.run()
		}()
	} else {
		d.run()
	}
	return nil
}

func (d *ExecuteStageController) run() {
	var meetError bool

	for stage := d.getCurrentStage(); stage != nil; stage = d.getCurrentStage() {
		//ignore finish stage
		if !stage.IsFinish {
			stage.StartTime = typeutil.Time(time.Now())
			d.setStage(d.currentStage, *stage)

			if stage.Run != nil {
				if err := stage.Run(d.ctx); err != nil {
					stage.ErrorMessage = err
					meetError = true
				} else {
					stage.Success = true
				}
			}

			stage.EndTime = typeutil.Time(time.Now())
			stage.IsFinish = true
			d.setStage(d.currentStage, *stage)
			if meetError || d.isStop {
				break
			}
		}
		d.setCurrentNextStage()

		if d.stageSaveFun != nil {
			if err := d.stageSaveFun(d.ctx); err != nil {
				stage.ErrorMessage = err
				meetError = true
				break
			}
		}
	}

	switch {
	case d.isStop:
		d.setState(StateStop)
	case meetError:
		d.setState(StateError)
	default:
		d.setState(StateComplete)
	}

	if d.stagesFinishFunc != nil {
		d.stagesFinishFunc(d.ctx)
	}
}

func (d *ExecuteStageController) setStage(index int, stage Stage) {
	d.stages[index] = stage
}

func (d *ExecuteStageController) getCurrentStage() *Stage {
	if d.currentStage == len(d.stages) {
		return nil
	}
	stage := d.stages[d.currentStage]
	return &stage
}

func (d *ExecuteStageController) setState(state string) {
	d.state = state
}

func (d *ExecuteStageController) GetState() string {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.state
}

func (d *ExecuteStageController) setCurrentNextStage() {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.currentStage += 1
}

func (d *ExecuteStageController) getState() string {
	return d.state
}

func (d *ExecuteStageController) RegisterStage(stage Stage) *ExecuteStageController {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.state == StateInitializing {
		d.stages = append(d.stages, stage)
	}
	return d
}

func (d *ExecuteStageController) RegisterStages(stages []Stage) *ExecuteStageController {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.state == StateInitializing {
		d.stages = append(d.stages, stages...)
	}
	return d
}

func (d *ExecuteStageController) SetSaveFun(sf func(ctx context.Context) error) *ExecuteStageController {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.state == StateInitializing {
		d.stageSaveFun = sf
	}
	return d
}

func (d *ExecuteStageController) SetFinishFunc(sf func(ctx context.Context) error) *ExecuteStageController {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.state == StateInitializing {
		d.stagesFinishFunc = sf
	}
	return d
}

func (d *ExecuteStageController) Stop() {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.state != StateRunning && !d.async {
		return
	}
	d.isStop = true
}

func (d *ExecuteStageController) GetStages() []Stage {
	d.lock.Lock()
	defer d.lock.Unlock()

	stages := make([]Stage, 0, len(d.stages))
	for _, v := range d.stages {
		stages = append(stages, v)
	}
	return stages
}
