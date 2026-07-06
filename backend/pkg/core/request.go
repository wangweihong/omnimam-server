package core

import (
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/validation"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

func DecodeParameter(c *gin.Context, obj any) error {
	if obj == nil {
		return nil
	}
	if d, ok := obj.(imachinery.Decoder); ok {
		if err := d.Decode(c); err != nil {
			//return errors.WrapStatus(err, code.ErrValidation)
			return errors.WrapCode(err, code.ErrValidation)
		}
	} else {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			mediaType, _, _ := mime.ParseMediaType(c.GetHeader("Content-Type"))
			if mediaType != "multipart/form-data" {
				if err := c.ShouldBindJSON(obj); err != nil {
					//	return errors.WrapStatus(err, code.ErrValidation)
					return errors.WrapCode(err, code.ErrValidation)
				}
			} else {
				// 注意读取的tag为form
				if err := c.ShouldBind(obj); err != nil {
					return errors.WrapCode(err, code.ErrValidation)
				}
			}

		case http.MethodGet:
			if err := c.ShouldBindQuery(obj); err != nil {
				//return errors.WrapStatus(err, code.ErrValidation)
				return errors.WrapCode(err, code.ErrValidation)
			}
		}
	}

	// 解析后需要处理一些空值默认值的情况
	if d, ok := obj.(imachinery.DefaultSetter); ok {
		d.SetDefaults()
	}

	// 自定义检测
	if d, ok := obj.(validation.Validator); ok {
		if err := d.Validate(); err != nil {
			return errors.WrapStatus(err, code.ErrValidation)
		}
	}

	if d, ok := obj.(imachinery.PostBinder); ok {
		if err := d.PostBind(); err != nil {
			return errors.WrapStatus(err, code.ErrBind)
		}
	}

	return nil
}
