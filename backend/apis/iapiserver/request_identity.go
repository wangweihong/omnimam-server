package iapiserver

import "github.com/wangweihong/omnimam/backend/apis/imachinery"

type (
	UserListRequest struct {
		imachinery.BasicQueryParam
	}

	UserListResponse struct {
		imachinery.ListRet
		List []*User `json:"list"`
	}
)

type (
	UserGetRequest struct {
		User
	}

	UserGetResponse struct {
		User
	}
)

type (
	UserAddRequest struct {
		User
	}

	UserAddResponse struct {
		User
	}
)

type (
	UserDeleteRequest struct {
		User
	}

	UserDeleteResponse struct {
		User
	}
)

type (
	UserUpdateRequest struct {
		User
	}

	UserUpdateResponse struct {
		User
	}
)
