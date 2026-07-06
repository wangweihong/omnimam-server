package regexutil

import (
	"regexp"
	"strings"
)

func ExtractNumbers(s string) string {
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(s, -1)
	return strings.Join(matches, "")
}
