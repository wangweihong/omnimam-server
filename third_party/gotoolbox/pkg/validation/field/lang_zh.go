package field

import "fmt"

var _ ErrorLangMessage = &errorLangCn{}

type errorLangCn struct{}

func (e errorLangCn) SupportMessage() string {
	return "支持值: "
}

func (e errorLangCn) TooLong(maxLength int) string {
	return fmt.Sprintf("必须小于%d位", maxLength)
}

func (e errorLangCn) TooManyMessage(maxQuantity int) string {
	return fmt.Sprintf("最多%d个元素", maxQuantity)
}

func (e errorLangCn) TypeMessage(t ErrorType) string {
	switch t {
	case ErrorTypeNotFound:
		return "未找到"
	case ErrorTypeRequired:
		return "必填值"
	case ErrorTypeDuplicate:
		return "重复值"
	case ErrorTypeInvalid:
		return "无效值"
	case ErrorTypeNotSupported:
		return "不支持的值"
	case ErrorTypeForbidden:
		return "禁止"
	case ErrorTypeTooLong:
		return "太长"
	case ErrorTypeTooMany:
		return "太多"
	case ErrorTypeInternal:
		return "内部错误"
	default:
		panic(fmt.Sprintf("未识别的验证错误: %q", string(t)))
	}
}
