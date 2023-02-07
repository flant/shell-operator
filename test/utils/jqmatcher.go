//go:build test
// +build test

package utils

import (
	"encoding/json"
	"fmt"

	"github.com/onsi/gomega/matchers"
	"github.com/onsi/gomega/types"

	. "github.com/flant/libjq-go"
)

// var _ = Ω(verBcs).To(MatchJq(`.[0] | has("objects")`, Equal(true)))
func MatchJq(jqExpression string, matcher interface{}) types.GomegaMatcher {
	return &matchJq{
		JqExpr:  jqExpression,
		Matcher: matcher,
	}
}

type matchJq struct {
	JqExpr      string
	InputString string
	Matcher     interface{}
	ResMatcher  types.GomegaMatcher
}

func (matcher *matchJq) Match(actual interface{}) (success bool, err error) {
	switch v := actual.(type) {
	case string:
		matcher.InputString = v
	case []byte:
		matcher.InputString = string(v)
	default:
		inputBytes, err := json.Marshal(v)
		if err != nil {
			return false, fmt.Errorf("MatchJq marshal object to json: %s\n", err)
		}
		matcher.InputString = string(inputBytes)
	}

	// nolint:typecheck // Ignore false positive: undeclared name: `Jq`.
	res, err := Jq().Program(matcher.JqExpr).Run(matcher.InputString)
	if err != nil {
		return false, fmt.Errorf("MatchJq apply jq expression: %s\n", err)
	}

	var isMatcher bool
	matcher.ResMatcher, isMatcher = matcher.Matcher.(types.GomegaMatcher)
	if !isMatcher {
		matcher.ResMatcher = &matchers.EqualMatcher{Expected: res}
	}

	return matcher.ResMatcher.Match(res)
}

func (matcher *matchJq) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Jq expression `%s` applied to '%s' not match: %v", matcher.JqExpr, matcher.InputString, matcher.ResMatcher.FailureMessage(actual))
}

func (matcher *matchJq) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Jq expression `%s` applied to '%s' should not match: %s", matcher.JqExpr, matcher.InputString, matcher.ResMatcher.NegatedFailureMessage(actual))
}
