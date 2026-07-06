package imachinery

import "github.com/gin-gonic/gin"

// 参数解析后的处理参数
// 如Get请求时需要传递slice,但直接传递slice会有问题，因此一般都会传递一个带分隔符的字符串，再通过
//
//	分隔符切换成数组
type PostBinder interface {
	PostBind() error
}

// 自定义参数解析器
type Decoder interface {
	Decode(c *gin.Context) error
}

// 执行完服务的数据处理, 如隐藏密码等
type PostRun interface {
	Transform() any
}

// 结构体设置一些默认值
type DefaultSetter interface {
	SetDefaults()
}
