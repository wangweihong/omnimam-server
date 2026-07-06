package validator

import (
	"fmt"
	"sync"

	"github.com/gin-gonic/gin/binding"
	validator "github.com/go-playground/validator/v10"
	"github.com/wangweihong/gotoolbox/pkg/validation"
	"github.com/wangweihong/gotoolbox/pkg/validation/field"
)

type Lang string

type customValidator struct {
	validator *validation.CustomValidator
	once      sync.Once
}

var _ binding.StructValidator = (*customValidator)(nil)

// validateStruct receives struct type
func (v *customValidator) ValidateStruct(obj any) error {
	errList := v.validator.Validate(obj)
	return errList.ToAggregate()
}

// validateStruct receives struct type
func (v *customValidator) Validate(obj any) error {
	errList := v.validator.Validate(obj)
	return errList.ToAggregate()
}

// validateStruct receives struct type
func (v *customValidator) validateStruct(obj any) error {
	errList := v.validator.Validate(obj)
	return errList.ToAggregate()
}

func (v *customValidator) Engine() any {
	v.lazyinit()
	return v.validator
}

func (v *customValidator) lazyinit() {
	v.once.Do(func() {
		v.validator.SetTagName("binding")
	})
}

func NewCustomValidator(lang string) *customValidator {
	val := validation.NewValidator()
	// 错误信息返回中文
	if lang == validation.LangZH {
		val.SetDefaultTranslater(validation.LangZH)
		field.SetLanguage(field.LanguageZH)
	}

	// register validator and translator
	registerValidator(val, "name", ValidateName, NameTranslator{})
	registerValidator(val, "description", ValidateDescription, DescriptionTranslator{})
	registerValidator(val, "url", ValidateURL, URLInvaliTranslator{})
	registerValidator(val, "port", ValidatePort, PortRangeTranslator{})
	registerValidator(val, "ports", ValidatePorts, PortsRangeTranslator{})
	registerValidator(val, "port_used", ValidatePort, PortUsedTranslator{})
	registerValidator(val, "dns", ValidateDNSName, DNSTranslator{})
	registerValidator(val, "cidr", ValidateCIDR, CIDRTranslator{})
	registerValidatorNoTrans(val, "ips", ValidateIPs)
	registerValidatorNoTrans(val, "ip", ValidateIP)
	// registerValidatorNoTrans(val, "namespaced", ValidateNamespaceScopeResource)
	// registerValidatorNoTrans(val, "clusterd", ValidateClusterScopeResource)

	return &customValidator{validator: val}
}

// registerValidator注册校验器和对应的错误翻译
func registerValidator(
	validator *validation.CustomValidator,
	tag string,
	validate func(fl validator.FieldLevel) bool,
	translator Translator,
) {
	if err := validator.RegisterValidation(tag, validate); err != nil {
		panic(fmt.Sprintf("Failed to register validation for tag '%s': %v", tag, err))
	}
	for _, lang := range []string{validation.LangEN, validation.LangZH} {
		trans := validator.GetTranslator(lang)

		var translation string
		switch lang {
		case validation.LangZH:
			translation = translator.ZH()
		case validation.LangEN:
			fallthrough
		default:
			translation = translator.EN()
		}

		err := validator.RegisterTranslation(
			tag,
			trans,
			validation.RegistrationFunc(tag, translation),
			validation.TranslateFunc,
		)
		if err != nil {
			panic(fmt.Sprintf("Failed to register %s translation for tag '%s': %v", lang, tag, err))
		}
	}
}

func registerValidatorNoTrans(
	validator *validation.CustomValidator,
	tag string,
	validate func(fl validator.FieldLevel) bool,
) {
	if err := validator.RegisterValidation(tag, validate); err != nil {
		panic(fmt.Sprintf("Failed to register validation for tag '%s': %v", tag, err))
	}
}

func RegisterValidatorNoTrans(
	tag string,
	validate func(fl validator.FieldLevel) bool,
) {
	registerValidatorNoTrans(cval.validator, tag, validate)
}

func Init(lang string) *customValidator {
	cval := NewCustomValidator(lang)
	//如果需要自定义验证器，可以在这里注册到 gin的校验器
	// 必须执行这一步, 否则无法使用binding tag作为校验器
	cval.Engine()
	// 更改gin binding的检测器, gin会在bindJson时进行参数检测
	binding.Validator = cval
	return cval
}

var (
	cval *customValidator
)

// nolint: gochecknoinits
func init() {
	cval = Init(validation.LangEN)
}
