package field

type ErrorLangMessage interface {
	SupportMessage() string
	TooLong(d int) string
	TooManyMessage(d int) string
	TypeMessage(t ErrorType) string
}

func NewErrorLangMessage(lang string) ErrorLangMessage {
	switch lang {
	case LanguageZH:
		return errorLangCn{}
	}
	return errorLangEn{}
}

const (
	LanguageEN = "en"
	LanguageZH = "zh"
)

var errorLanguage = LanguageEN

func SetLanguage(lang string) {
	switch lang {
	case LanguageEN, LanguageZH:
		errorLanguage = lang
	default:
		errorLanguage = LanguageEN
	}
}
