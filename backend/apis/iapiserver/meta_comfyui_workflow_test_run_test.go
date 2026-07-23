package iapiserver

import (
	"encoding/json"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestComfyUIWorkflowTestRunSnapshots(t *testing.T) {
	Convey("试运行快照应持久化并恢复公开摘要", t, func() {
		region := "local"
		run := &ComfyUIWorkflowTestRun{
			EngineInstanceSnapshot: ComfyUIWorkflowTestEngineSnapshot{ID: "engine-a", Name: "ComfyUI-A", Region: &region},
			Parameters:             []ComfyUIWorkflowTestParameter{{NodeID: "3", InputName: "seed", Value: float64(42)}},
			OutputSelections:       []ComfyUIWorkflowTestOutputSelection{{NodeID: "9", OutputIndex: 0}},
			Steps:                  defaultSnapshotTestSteps(),
			Outputs:                []ComfyUIWorkflowTestOutput{},
		}

		So(run.marshal(), ShouldBeNil)
		So(run.ParameterOverrideCount, ShouldEqual, 1)
		So(run.EngineSnapshotShadow, ShouldContainSubstring, "ComfyUI-A")
		So(run.ParametersShadow, ShouldContainSubstring, "seed")
		So(run.OutputSelectionsShadow, ShouldContainSubstring, `"node_id":"9"`)

		loaded := &ComfyUIWorkflowTestRun{
			EngineSnapshotShadow:   run.EngineSnapshotShadow,
			ParametersShadow:       run.ParametersShadow,
			OutputSelectionsShadow: run.OutputSelectionsShadow,
			StepsShadow:            run.StepsShadow,
			OutputsShadow:          run.OutputsShadow,
			WorkflowSnapshotShadow: "{}",
		}
		So(loaded.AfterFind(nil), ShouldBeNil)
		So(loaded.EngineInstanceSnapshot.Name, ShouldEqual, "ComfyUI-A")
		So(loaded.ParameterOverrideCount, ShouldEqual, 1)
		So(loaded.Parameters[0].InputName, ShouldEqual, "seed")
		So(loaded.OutputSelections, ShouldResemble, []ComfyUIWorkflowTestOutputSelection{{NodeID: "9", OutputIndex: 0}})
	})

	Convey("轻量列表投影应保持同一结构并显式返回 null 复杂字段", t, func() {
		run := &ComfyUIWorkflowTestRun{
			EngineInstanceSnapshot: ComfyUIWorkflowTestEngineSnapshot{ID: "engine-a", Name: "ComfyUI-A"},
			ParameterOverrideCount: 3,
			Parameters:             nil,
			OutputSelections:       nil,
			Steps:                  nil,
			Outputs:                nil,
		}
		payload, err := json.Marshal(run)
		So(err, ShouldBeNil)
		value := string(payload)
		So(value, ShouldContainSubstring, `"parameter_override_count":3`)
		So(value, ShouldContainSubstring, `"parameter_snapshot":null`)
		So(value, ShouldContainSubstring, `"output_snapshot":null`)
		So(value, ShouldContainSubstring, `"steps":null`)
		So(value, ShouldContainSubstring, `"outputs":null`)
		So(strings.Contains(value, "base_url"), ShouldBeFalse)
	})
}

func defaultSnapshotTestSteps() []ComfyUIWorkflowTestStep {
	return []ComfyUIWorkflowTestStep{{Key: "submit", Label: "提交", Status: "PENDING"}}
}
