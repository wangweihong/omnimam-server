package validation

import (
	"fmt"
	"os"
	"reflect"

	"github.com/go-playground/locales/en"
	zh "github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	validator "github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	zh_translations "github.com/go-playground/validator/v10/translations/zh"
	"github.com/wangweihong/gotoolbox/pkg/validation/field"
)

const (
	maxDescriptionLength = 255

	LangEN = "en"
	LangZH = "zh"
)

// Validator is a custom validator for configs.
type CustomValidator struct {
	val   *validator.Validate
	uni   *ut.UniversalTranslator
	trans ut.Translator
	lang  string
}

// NewValidator creates a new Validator with default translation.
func NewValidator() *CustomValidator {
	result := validator.New()

	// default translations
	enLocale := en.New()
	zhLocale := zh.New()

	uni := ut.New(enLocale, enLocale, zhLocale)

	// Register default translations for English and Chinese
	registerDefaultTranslations(result, uni, LangEN, en_translations.RegisterDefaultTranslations)
	registerDefaultTranslations(result, uni, LangZH, zh_translations.RegisterDefaultTranslations)

	defaultTranslator, _ := uni.GetTranslator(LangEN)
	return &CustomValidator{
		val:   result,
		uni:   uni,
		trans: defaultTranslator,
		lang:  LangEN,
	}
}

// registerDefaultTranslations registers default translations for a given language
func registerDefaultTranslations(validate *validator.Validate, uni *ut.UniversalTranslator, lang string, registerFunc func(*validator.Validate, ut.Translator) error) {
	trans, _ := uni.GetTranslator(lang)
	if err := registerFunc(validate, trans); err != nil {
		panic(fmt.Sprintf("Failed to register %s translations: %v", lang, err))
	}
}

func registerCustomValidators(validate *validator.Validate, uni *ut.UniversalTranslator) {
	validate.RegisterValidation("dir", ValidateDir)                 // nolint: errcheck // no need
	validate.RegisterValidation("file", ValidateFile)               // nolint: errcheck // no need
	validate.RegisterValidation("description", ValidateDescription) // nolint: errcheck // no need
	validate.RegisterValidation("name", ValidateName)               // nolint: errcheck // no need

	registerCustomTranslations(validate, uni)
}

func registerCustomTranslations(validate *validator.Validate, uni *ut.UniversalTranslator) {
	translations := []struct {
		tag           string
		enTranslation string
		zhTranslation string
	}{
		{
			tag:           "dir",
			enTranslation: "{0} must point to an existing directory, but found '{1}'",
			zhTranslation: "{0}必须指向一个存在的目录，但找到了  '{1}'",
		},
		{
			tag:           "file",
			enTranslation: "{0} must point to an existing file, but found '{1}'",
			zhTranslation: "{0}必须指向一个存在的文件，但找到了'{1}'",
		},
		{
			tag:           "description",
			enTranslation: fmt.Sprintf("must be less than %d characters", 256),
			zhTranslation: fmt.Sprintf("必须少于 %d 个字符", 256),
		},
		{
			tag:           "name",
			enTranslation: "is not a valid name",
			zhTranslation: "不是一个有效的名称",
		},
	}

	// Register translations for each language
	for _, lang := range []string{LangEN, LangZH} {
		trans, _ := uni.GetTranslator(lang)
		for _, t := range translations {
			translation := t.enTranslation
			if lang == LangZH {
				translation = t.zhTranslation
			}
			err := validate.RegisterTranslation(t.tag, trans, RegistrationFunc(t.tag, translation), TranslateFunc)
			if err != nil {
				panic(fmt.Sprintf("Failed to register %s translation for tag '%s': %v", lang, t.tag, err))
			}
		}
	}
}

func RegistrationFunc(tag string, translation string) validator.RegisterTranslationsFunc {
	return func(ut ut.Translator) (err error) {
		if err = ut.Add(tag, translation, true); err != nil {
			return
		}

		return
	}
}

func TranslateFunc(ut ut.Translator, fe validator.FieldError) string {
	t, err := ut.T(fe.Tag(), fe.Field(), reflect.ValueOf(fe.Value()).String())
	if err != nil {
		return fe.(error).Error()
	}

	return t
}

// 注册内置默认的校验器, 包括对应的错误翻译
func (v *CustomValidator) SetDefaultCustomValidator() *CustomValidator {
	registerCustomValidators(v.val, v.uni)
	return v
}

// SetDefaultTranslater 设置默认翻译器
func (v *CustomValidator) SetDefaultTranslater(lang string) *CustomValidator {
	translator, found := v.uni.GetTranslator(lang)
	if found {
		v.trans = translator
		v.lang = lang
		// 如果是中文, 则使用`comment`替换英文字段名称，没有comment字段就默认字段
		// 注意这里有个Bug/机制: Validator在Struct()调用后，再RegisterTagNameFunc()不生效。
		if lang == LangZH {
			// https://github.com/LinkinStars/golang-web-template/blob/master/src/gwt/base/validator/validator.go
			// https://github.com/go-playground/validator/issues/524
			// 收集结构体中的comment标签，用于替换英文字段名称，这样返回错误就能展示中文字段名称了
			v.val.RegisterTagNameFunc(func(fld reflect.StructField) string {
				return fld.Tag.Get("comment")
			})
		}
		return v
	}

	fmt.Println("lang not found")

	return v
}

// RegisterValidation 添加自定义校验器
func (v *CustomValidator) RegisterValidation(tag string, fn validator.Func, callValidationEvenIfNull ...bool) error {
	return v.val.RegisterValidation(tag, fn, callValidationEvenIfNull...)
}

func (v *CustomValidator) GetTranslator(lang string) ut.Translator {
	trans, exist := v.uni.GetTranslator(lang)
	if !exist {
		//fall back to en
		trans, _ = v.uni.GetTranslator(LangEN)
	}
	return trans
}

// RegisterValidation 添加自定义校验器错误翻译
// 配合RegisterValidation一起使用。 RegisterValidation注册的校验器，只会检测是否成功。依赖于RegisterTranslation注册对应的检验错误信息
func (v *CustomValidator) RegisterTranslation(tag string, trans ut.Translator, registerFn validator.RegisterTranslationsFunc, translationFn validator.TranslationFunc) error {
	return v.val.RegisterTranslation(tag, trans, registerFn, translationFn)
}

func (v *CustomValidator) SetTagName(tag string) {
	v.val.SetTagName(tag)
}

// Validate validates config for errors and returns an error (it can be casted to
// ValidationErrors, containing a list of errors inside). When error is printed as string, it will
// automatically contains the full list of validation errors.
func (v *CustomValidator) Validate(data any) field.ErrorList {
	// validate policy
	err := v.val.Struct(data)
	if err == nil {
		return nil
	}

	// this check is only needed when your code could produce
	// an invalid value for validation such as interface with nil
	// value most including myself do not usually have code like this.
	if _, ok := err.(*validator.InvalidValidationError); ok { //nolint: errorlint
		return field.ErrorList{field.Invalid(field.NewPath(""), err.Error(), "")}
	}

	allErrs := field.ErrorList{}

	// collect human-readable errors
	vErrors, _ := err.(validator.ValidationErrors) //nolint: errorlint
	for _, vErr := range vErrors {
		// vErr.Namespace()会返回具体是哪个结构体的哪个报错结构字段,如AddUserReq.Sub.UserName
		// 如果翻译成中文, 且设置了comment字段，则会变成AddUserReq.Sub.用户名。 这不符合需要
		// 因此如果是翻译成中文，则去掉父字段，只保留报错字段信息"用户名"
		name := vErr.Namespace()
		if v.lang == LangZH {
			name = vErr.Field()
		}
		allErrs = append(allErrs, field.Invalid(field.NewPath(name), vErr.Translate(v.trans), ""))
	}

	return allErrs
}

// ValidateDir checks if a given string is an existing directory.
func ValidateDir(fl validator.FieldLevel) bool {
	path := fl.Field().String()
	if stat, err := os.Stat(path); err == nil && stat.IsDir() {
		return true
	}

	return false
}

// ValidateFile checks if a given string is an existing file.
func ValidateFile(fl validator.FieldLevel) bool {
	path := fl.Field().String()
	if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
		return true
	}

	return false
}

// ValidateDescription checks if a given description is illegal.
func ValidateDescription(fl validator.FieldLevel) bool {
	description := fl.Field().String()

	return len(description) <= maxDescriptionLength
}

// ValidateName checks if a given name is illegal.
func ValidateName(fl validator.FieldLevel) bool {
	name := fl.Field().String()
	if errs := IsQualifiedName(name); len(errs) > 0 {
		return false
	}

	return true
}
