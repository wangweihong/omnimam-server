package callstatus

import (
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

// ToError convert grpc call status to err.
func ToError(cs *CallStatus) *errors.Status {
	if cs == nil || cs.Code == 0 || int(cs.Code) == code.ErrSuccess {
		return nil
	}

	// TODO: fixme
	return &errors.Status{
		// HTTPStatus: ,
		Code:    int(cs.Code),
		Message: cs.Message,
		Desc:    cs.Description,
		// Cause:      ,
	}
}

// FromError convert err to grpc call status.
func FromError(err error) *CallStatus {
	if err == nil {
		return &CallStatus{}
	}
	st := errors.ToStatus(err)
	return &CallStatus{
		Code:        int64(st.Code),
		Message:     st.Message,
		Description: st.Desc,
	}
}
