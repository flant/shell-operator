package string_helper

import (
	"regexp"
	"strings"
)

var safeReList = []*regexp.Regexp{
	regexp.MustCompile(`([A-Z])`),
	regexp.MustCompile(`[^a-z0-9-/]`),
	regexp.MustCompile(`[-]+`),
}

func SafeURLString(s string) string {
	s = safeReList[0].ReplaceAllString(s, "-$1")
	s = strings.ToLower(s)
	s = safeReList[1].ReplaceAllString(s, "-")
	return safeReList[2].ReplaceAllString(s, "-")
}
