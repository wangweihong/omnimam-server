package postgresql

import (
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

// CountAndFindPage 统一执行列表总数统计和分页查询，query 不得预先应用 offset/limit。
func CountAndFindPage[T any](query *gorm.DB, params imachinery.PagingParams, items *[]T) (int64, error) {
	window, err := params.Normalize()
	if err != nil {
		return 0, err
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return 0, errors.WithStack(err)
	}
	if err := imachinery.ApplyPagination(query, window).Find(items).Error; err != nil {
		return 0, errors.WithStack(err)
	}
	return total, nil
}
