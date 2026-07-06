package generic

type Number interface {
	int | int32 | uint32 | int64 | uint64 | float32 | float64
}

type Int interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint32 | uint64
}

// Ordered 支持<,<=,>,>=运算符, Ordered的类型均支持comparable
type Ordered interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint32 | uint64 | float32 | float64 | string
}

// comparable仅用于约束==和!=
// map,slice,function,chan,any 类型不支持comparable
type comparable interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint32 | uint64 | float32 | float64 | string
}