package cryptoutil

import (
	"crypto/md5"
	"fmt"
	"io"
)

func Md5Encrypt(data string) (string, error) {
	h := md5.New()
	if _, err := io.WriteString(h, data); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil

}
