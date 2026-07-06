package imachinery

type BatchOutput struct {
	// 批量执行总数
	Total int `json:"total"`
	// 执行成功数量
	Success int `json:"success"`
	// 执行失败数量
	Fail int `json:"fail"`
	// 每次执行的
	Results []*Output `json:"results"`
}

// Output 输出
type Output struct {
	error
	Data any `json:"data,omitempty"`
}

func SetOutput(data any, err error) *Output {
	return &Output{
		Data:  data,
		error: err,
	}
}
