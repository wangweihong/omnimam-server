package assetlibrary

import (
	"strings"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

const thumbnailListProfile = "list-320"

// RepresentationPolicy 由 asset-library 消费，按媒体类型和 profile version 形成 expected set。
type RepresentationPolicy interface {
	Plan(mediaType, profileVersion string) store.RepresentationPlan
}

// DefaultRepresentationPolicy 实现 released SSOT 的首期 image/video 缩略图策略。
type DefaultRepresentationPolicy struct{}

func (DefaultRepresentationPolicy) Plan(mediaType, profileVersion string) store.RepresentationPlan {
	plan := store.RepresentationPlan{MediaType: mediaType, ProfileVersion: strings.TrimSpace(profileVersion), ExpectedCount: 1}
	if mediaType == iapiserver.AssetMediaTypeImage || mediaType == iapiserver.AssetMediaTypeVideo {
		plan.ExpectedCount = 2
		plan.Requested = []store.ExpectedRepresentation{{Type: "thumbnail", Profile: thumbnailListProfile, Required: false}}
	}
	return plan
}
