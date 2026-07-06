package decoder

import (
	"encoding/base64"
)

func MustBase64Decode(origin string) []byte {
	d, _ := base64.StdEncoding.DecodeString(origin)
	return d
}

func MustBase64Encode(origin []byte) string {
	return base64.StdEncoding.EncodeToString(origin)
}
