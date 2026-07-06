package ginx

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

// Get Raw Data from gin context.
func ParseRawData(c *gin.Context) ([]byte, error) {
	raw, err := c.GetRawData()
	if err != nil {
		log.F(c).Errorf("get raw data:%v", err)
		return nil, errors.WrapCode(err, code.ErrBind)
	}

	log.F(c).Debug("get raw data:", log.Every("req", string(raw)))
	return raw, nil
}

// Parse body json data to struct.
func ParseJSON(c *gin.Context, obj any) error {
	if err := c.ShouldBindJSON(obj); err != nil {
		log.F(c).Errorf("pares json data:%v", err)
		return errors.WrapCode(err, code.ErrBind)
	}
	log.F(c).Debug("parse json data:", log.Every("req", obj))
	return nil
}

// Parse query parameter to struct.
func ParseQuery(c *gin.Context, obj any) error {
	if err := c.ShouldBindQuery(obj); err != nil {
		log.F(c).Errorf("pares query data:%v", err)
		return errors.WrapCode(err, code.ErrBind)
	}
	log.F(c).Debug("parse query data:", log.Every("req", obj))

	return nil
}

// Parse body form data to struct.
func ParseForm(c *gin.Context, obj any) error {
	if err := c.ShouldBindWith(obj, binding.Form); err != nil {
		log.F(c).Errorf("pares form data:%v", err)
		return errors.WrapCode(err, code.ErrBind)
	}
	log.F(c).Debug("parse form body:", log.Every("req", obj))

	return nil
}
