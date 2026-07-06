package field

import "fmt"

var _ ErrorLangMessage = &errorLangEn{}

type errorLangEn struct{}

func (e errorLangEn) SupportMessage() string {
	return "supported values: "
}

func (e errorLangEn) TooLong(maxLength int) string {
	return fmt.Sprintf("must have at most %d bytes", maxLength)
}

func (e errorLangEn) TooManyMessage(maxQuantity int) string {
	return fmt.Sprintf("must have at most %d items", maxQuantity)
}

func (e errorLangEn) TypeMessage(t ErrorType) string {
	switch t {
	case ErrorTypeNotFound:
		return "Not found"
	case ErrorTypeRequired:
		return "Required value"
	case ErrorTypeDuplicate:
		return "Duplicate value"
	case ErrorTypeInvalid:
		return "Invalid value"
	case ErrorTypeNotSupported:
		return "Unsupported value"
	case ErrorTypeForbidden:
		return "Forbidden"
	case ErrorTypeTooLong:
		return "Too long"
	case ErrorTypeTooMany:
		return "Too many"
	case ErrorTypeInternal:
		return "Internal error"
	default:
		panic(fmt.Sprintf("unrecognized validation error: %q", string(t)))
	}
}
