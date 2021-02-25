package string_helper

import "strings"

// TrimGroup removes prefix with a slash.
//
// E.g. input string "stable.example.com/v1beta1"
// gives "v1beta1".
func TrimGroup(apiVersion string) string {
	idx := strings.IndexRune(apiVersion, '/')
	if idx >= 0 {
		//
		return apiVersion[idx+1:]
	}
	return apiVersion
}
