package waitgroup

type Output struct {
	Data any   `json:"data"`
	Err  error `json:"err"`
}

type BatchOutput struct {
	Total   int       `json:"total"`
	Success int       `json:"success"`
	Fail    int       `json:"fail"`
	Results []*Output `json:"results"`
}

func (b *BatchOutput) SetOutput(data any, err error) {
	b.Total += 1
	if err != nil {
		b.Fail += 1
		b.Results = append(b.Results, &Output{Data: data, Err: err})
		return
	}

	b.Success += 1
	b.Results = append(b.Results, &Output{Data: data, Err: err})
}

func (b *BatchOutput) Merge(c *BatchOutput) {
	if c == nil {
		return
	}

	b.Total += c.Total
	b.Success += c.Success
	b.Fail += c.Fail

	if c.Results != nil {
		b.Results = append(b.Results, c.Results...)
	}
}

func SetOutput(data any, err error) *Output {
	var MyOutput Output
	MyOutput.Err = err
	if data == nil {
		MyOutput.Data = ""
	} else {
		MyOutput.Data = data
	}
	return &MyOutput
}

type GenericOutput[T any] struct {
	Data T     `json:"data"`
	Err  error `json:"err"`
}

type BatchGenericOutput[T any] struct {
	Total   int                 `json:"total"`
	Success int                 `json:"success"`
	Fail    int                 `json:"fail"`
	Results []*GenericOutput[T] `json:"results"`
}

func SetGenericOutput[T any](data T, err error) *GenericOutput[T] {
	var MyOutput GenericOutput[T]
	MyOutput.Err = err
	MyOutput.Data = data
	return &MyOutput
}

func SetBatchGenericOutput[T any](b BatchGenericOutput[T], data T, err error) {
	b.Total += 1
	if err != nil {
		b.Fail += 1
		b.Results = append(b.Results, SetGenericOutput(data, err))
		return
	}

	b.Success += 1
	b.Results = append(b.Results, SetGenericOutput(data, err))
}
