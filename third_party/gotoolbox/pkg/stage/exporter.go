package stage

type ControllerExporter struct {
	Name         string  `json:"name"`
	Stages       []Stage `json:"stages"`
	CurrentStage int     `json:"current_stage"`
	State        string  `json:"state"`
	IsStop       bool    `json:"is_stop"`
}
