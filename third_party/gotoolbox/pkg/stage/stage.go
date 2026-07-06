package stage

import (
	"context"
	"time"
)

type Stage struct {
	Name         string                          `json:"name"`
	NameCn       string                          `json:"name_cn"`
	Desc         string                          `json:"desc"`
	StartTime    *time.Time                      `json:"start_time"`
	EndTime      *time.Time                      `json:"end_time"`
	Success      bool                            `json:"success"`
	ErrorMessage error                           `json:"error"`
	IsFinish     bool                            `json:"is_finish"`
	Run          func(ctx context.Context) error `json:"-"`
}

func NewExecuteStage(name, nameCN string, Run func(ctx context.Context) error) Stage {
	return Stage{
		Name:   name,
		NameCn: nameCN,
		Run:    Run,
	}
}
