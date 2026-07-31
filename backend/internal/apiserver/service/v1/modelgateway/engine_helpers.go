package modelgateway

import (
	stderrors "errors"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/typeutil"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/helpers"
)

func applyEngineUpdate(item *iapiserver.EngineInstance, req *iapiserver.EngineInstanceUpdateRequest) {
	helpers.ApplyString(&item.Name, req.Name)
	helpers.ApplyString(&item.Description, req.Description)
	helpers.ApplyString(&item.BaseURL, req.BaseURL)
	helpers.ApplyString(&item.AuthType, req.AuthType)
	if req.AuthConfig != nil {
		item.AuthConfig = req.AuthConfig
	}
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	helpers.ApplyString(&item.Region, req.Region)
	helpers.ApplyInt(&item.MaxConcurrency, req.MaxConcurrency)
	helpers.ApplyInt(&item.RequestTimeoutSeconds, req.RequestTimeoutSeconds)
	helpers.ApplyInt(&item.TaskTimeoutSeconds, req.TaskTimeoutSeconds)
}

func validateRestrictions(capability *iapiserver.AIAppProviderCapability, restrictions map[string]any) error {
	if len(restrictions) == 0 {
		return nil
	}
	allowed := map[string]map[string]struct{}{"model_ids": {}, "operation_ids": {}, "variant_ids": {}}
	for _, item := range capability.Models {
		allowed["model_ids"][item.ID] = struct{}{}
	}
	for _, item := range capability.Operations {
		allowed["operation_ids"][item.ID] = struct{}{}
	}
	for _, item := range capability.Variants {
		allowed["variant_ids"][item.ID] = struct{}{}
	}
	for key, value := range restrictions {
		set, ok := allowed[key]
		if !ok {
			return errors.NewStatus(code.ErrAIAppEngineBindingRestrictionExpands, "unknown restriction key "+key)
		}
		for _, item := range typeutil.SliceAs[string](value) {
			if _, ok := set[item]; !ok {
				return errors.NewStatus(code.ErrAIAppEngineBindingRestrictionExpands, "restriction adds unknown value "+item)
			}
		}
	}
	return nil
}

func mapNotFound(err error, businessCode int, message string) error {
	if err == nil {
		return nil
	}
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatus(businessCode, message)
	}
	return err
}

func mapUnique(err error, constraint string, businessCode int, message string) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), constraint) || strings.Contains(err.Error(), "duplicate key") {
		return errors.NewStatus(businessCode, message)
	}
	return err
}

func defaultInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
