package errors

import (
	"fmt"
	"strings"
)

func TrimError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%v", strings.Replace(err.Error(), "\n", " ", -1))
}
