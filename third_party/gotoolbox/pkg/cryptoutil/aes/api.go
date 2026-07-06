package aes

const (
	AesKey    = "1~$c31kjtR^@@c2#"
	AesVector = "#$3456$890A54321"
)

func EncyptPassword(origin string) (string, error) {
	crypto := NewCrypto(StandardType.AES, ModeType.CBC, PaddingType.ZERO, FormatType.HEX)
	encryptoData, err := crypto.Encrypt([]byte(origin), []byte(AesKey), []byte(AesVector))
	if err != nil {
		return "", err
	}

	return encryptoData, nil
}

func DecryptPassword(origin string) (string, error) {
	crypto := NewCrypto(StandardType.AES, ModeType.CBC, PaddingType.ZERO, FormatType.HEX)
	decryptoData, err := crypto.Decrypt([]byte(origin), []byte(AesKey), []byte(AesVector))
	if err != nil {
		return "", err
	}

	return decryptoData, nil
}
