package core_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

func TestRun(t *testing.T) {
	err := errors.New("tst")
	//err := fmt.Errorf("xxx")
	err = errors.WrapCode(err, code.ErrBind)
	//err = errors.WrapStatus(err, code.ErrValidation)
	log.Errorf("%#+v", err)
}
