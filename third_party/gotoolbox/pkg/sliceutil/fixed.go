package sliceutil

/*

graph LR
A[添加元素] --> B{是否已满?}
B -->|否| C[追加到尾部]
B -->|是| D[替换最旧元素]
D --> E[移动起始指针]

*/
// FixedSlice 固定长度切片
type FixedSlice[T any] struct {
	data   []T // 存储元素的切片
	maxLen int // 最大允许长度
	start  int // 有效数据的起始索引（用于环形缓冲区）
	count  int // 当前元素数量
}

// NewFixedSlice 创建新的固定长度切片
func NewFixedSlice[T any](maxLen int) *FixedSlice[T] {
	if maxLen <= 0 {
		panic("maxLen must be positive")
	}
	return &FixedSlice[T]{
		data:   make([]T, 0, maxLen*2), // 初始容量为maxLen*2以优化性能
		maxLen: maxLen,
	}
}

// Append 添加元素，如果超过最大长度则移除最旧元素
func (fs *FixedSlice[T]) Append(item T) {
	// 计算写入位置

	writePos := (fs.start + fs.count) % fs.maxLen
	if fs.count < fs.maxLen {
		// 切片未满，直接写入
		fs.data = append(fs.data, item)
		fs.count++
	} else {
		// 切片已满，覆盖最旧元素
		fs.data[writePos] = item
		// 移动起始位置（移除最旧元素）
		fs.start = (fs.start + 1) % fs.maxLen
	}
}

// GetAll 获取所有元素（按添加顺序）
func (fs *FixedSlice[T]) GetAll() []T {
	if fs.count == 0 {
		return nil
	}

	result := make([]T, fs.count)

	if fs.start+fs.count <= fs.maxLen {
		// 连续内存块
		copy(result, fs.data[fs.start:fs.start+fs.count])
	} else {
		// 环形缓冲区，分两段复制
		firstPart := fs.data[fs.start:]
		copy(result, firstPart)

		remaining := fs.count - len(firstPart)
		copy(result[len(firstPart):], fs.data[:remaining])
	}

	return result
}

// Len 返回当前元素数量
func (fs *FixedSlice[T]) Len() int {
	return fs.count
}

// Cap 返回容量
func (fs *FixedSlice[T]) Cap() int {
	return fs.maxLen
}

// Clear 清空切片
func (fs *FixedSlice[T]) Clear() {
	fs.data = fs.data[:0]
	fs.start = 0
	fs.count = 0
}
