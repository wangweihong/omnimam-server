package stage

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"
)

var (
	c = NewExecuteStageController(context.Background(), "example").
		RegisterStage(NewExecuteStage("exampl1", "例子1", func(ctx context.Context) error {
			time.Sleep(1 * time.Second)
			fmt.Println("this is example1")
			return nil
		})).RegisterStage(NewExecuteStage("example2", "例子3", func(ctx context.Context) error {
		time.Sleep(3 * time.Second)
		fmt.Println("this is example2")
		return nil
	}))

	errStage = NewExecuteStage("error", "错误", func(ctx context.Context) error {
		time.Sleep(1)
		return fmt.Errorf("i am error")
	})
)

func TestNewExecuteStageController(t *testing.T) {
	if len(c.GetStages()) != 2 {
		t.Fail()
	}

	if c.state != StateInitializing {
		t.Fail()
	}

}

func TestNewExecuteStageControllerRun2(t *testing.T) {
	if err := c.Run(); err != nil {
		t.Log(err)
		t.Fail()
	}

	if err := c.Run(); err == nil {
		t.Log("re run success")
	} else {
		t.Log(err)
	}

	for _, v := range c.GetStages() {
		fmt.Printf("%s %s %v %v %v\n", v.Name, v.NameCn, v.StartTime, v.EndTime, v.IsFinish)
	}
}

func TestNewExecuteStageControllerRunError(t *testing.T) {
	if err := c.RegisterStage(errStage).Run(); err != nil {
		log.Fatal(err)
	}

	if c.state != StateError {
		log.Fatalf("no error state")
	}

	if c.GetError() == nil {
		log.Fatalf("no error return")
	}

	fmt.Println(c.GetError())
}

func TestNewExecuteStageControllerStopRun(t *testing.T) {
	if err := c.Run(); err != nil {
		t.Fatalf("controller run fail:%v", err)
	}

	if c.state != StateRunning {
		t.Fatalf("runnging controller state is not running")
	}

	c.Stop()

	if c.state != StateStop {
		t.Fatalf("stop controller state is not stop")
	}

	if err := c.Run(); err != nil {
		t.Fatalf("stop controller state is not stop")
	}
}
