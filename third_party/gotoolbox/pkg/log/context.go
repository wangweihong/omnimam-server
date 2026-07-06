package log

import (
	"context"
)

type (
	LoggerKeyCtx struct{} // store logger in context
	FieldKeyCtx  struct{} // store fields in context
)

// WithContext returns a copy of context in which the log value is set.
func WithContext(ctx context.Context) context.Context {
	return std.WithContext(ctx)
}

// save log handler into zap.
func (l *zapLogger) WithContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.TODO()
	}

	return context.WithValue(ctx, LoggerKeyCtx{}, l)
}

// FromContext returns the value of the log key on the ctx.
func FromContext(ctx context.Context) Logger {
	if ctx != nil {
		logger := ctx.Value(LoggerKeyCtx{})
		if logger != nil {
			return logger.(Logger)
		}
	}

	return WithName("Unknown-Context")
}

// 这里需要特别注意context的值传递和值获取机制. context只能自顶向下传值。
// child1 = context.WithValue(parent,k,v)调用时, child相当于在对parent拷贝, 并对其覆盖一层k/v
// 当child1.Value(k)时, 如果child1.k匹配, 则直接返回v。否则逐级向上比对父/祖先的k直到到达顶部或者有一个祖先匹配(valueCtx)。
// 这意味着子/父同key则取子, 兄弟之间彼此独立。
// 如果v为一个map/slice时, 如果每次修改从ctx.Value获取的map时,除非通过conext.WithValue()替换一个新的map,否则修改的时同一个map/slice。
// 这里采用的是复制父辈的fields, 相互隔离不影响。因此尽可能不要在fieldsCtx中存放过多的数据，
// WithFields returns a copy of context which inject fields to FieldKeyCtx. If parent has FieldKeyCtx, copy it into
// current one.
func WithFields(ctx context.Context, fields map[string]any) context.Context {
	if ctx == nil {
		ctx = context.TODO()
	}

	if fields == nil {
		return ctx
	}

	fieldMap := make(map[string]any)
	for k, v := range fields {
		fieldMap[k] = v
	}

	if originFields := ctx.Value(FieldKeyCtx{}); originFields != nil {
		if parentFieldMap, ok := originFields.(map[string]any); ok {
			for k, v := range parentFieldMap {
				fieldMap[k] = v
			}
		}
	}
	return context.WithValue(ctx, FieldKeyCtx{}, fieldMap)
}

// WithFieldPair returns a copy of context which inject key and value to fieldCtx.
func WithFieldPair(ctx context.Context, key string, value any) context.Context {
	if ctx == nil {
		ctx = context.TODO()
	}

	if key == "" {
		return ctx
	}

	fieldMap := make(map[string]any)
	if fields := ctx.Value(FieldKeyCtx{}); fields != nil {
		// copy parent fieldmap, don't bother parent fieldmap
		if parentFieldMap, ok := fields.(map[string]any); ok {
			for k, v := range parentFieldMap {
				fieldMap[k] = v
			}
		}
	}
	fieldMap[key] = value

	return context.WithValue(ctx, FieldKeyCtx{}, fieldMap)
}

func (c FieldKeyCtx) String() string {
	return "FieldKeyCtx"
}
