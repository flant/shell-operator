//go:build test
// +build test

package utils

import (
	"fmt"
	"strings"

	"github.com/onsi/gomega/types"
)

type JsonLogAnalyzer struct {
	Matchers      [][]*JsonLogMatcher
	OnStopFn      func()
	Index         int
	finished      bool
	groupFinished bool // flag to reset group of matchers after previous group finish
	err           error
}

func NewJsonLogAnalyzer() *JsonLogAnalyzer {
	return &JsonLogAnalyzer{
		Matchers:      make([][]*JsonLogMatcher, 0),
		Index:         0,
		finished:      false,
		groupFinished: true,
	}
}

func (a *JsonLogAnalyzer) AddGroup(analyzers ...*JsonLogMatcher) {
	a.Matchers = append(a.Matchers, analyzers)
}

func (a *JsonLogAnalyzer) OnStop(fn func()) {
	a.OnStopFn = fn
}

func (a *JsonLogAnalyzer) Finished() bool {
	return a.finished
}

func (a *JsonLogAnalyzer) Error() error {
	return a.err
}

func (a *JsonLogAnalyzer) Reset() {
	for _, matchers := range a.Matchers {
		for _, matcher := range matchers {
			matcher.Reset()
		}
	}
	a.Index = 0
	a.groupFinished = true
	a.finished = false
	a.err = nil
}

func (a *JsonLogAnalyzer) HandleLine(line string) {
	// fmt.Printf("Got line: %s\n", line)
	// defer func() {
	//	fmt.Printf("analyzer.HandleLine done: ind=%d fin=%v err=%v\n", a.Index, a.finished, a.err)
	// }()
	if a.finished {
		return
	}

	matchers := a.Matchers[a.Index]
	if a.groupFinished {
		for _, matcher := range matchers {
			matcher.Reset()
		}
		a.groupFinished = false
	}

	groupFinished := true
	for _, matcher := range matchers {
		matcher.HandleLine(line)
		if matcher.Error() != nil {
			a.err = matcher.Error()
			a.finished = true
			break
		}
		if !matcher.Finished() {
			groupFinished = false
		}
	}

	if !a.finished {
		if groupFinished {
			a.Index++
			a.groupFinished = groupFinished
		}
		if a.Index == len(a.Matchers) {
			a.finished = true
		}
	}

	if a.finished && a.OnStopFn != nil {
		// fmt.Printf("analyzer.HandleLine OnStopFn()\n")
		a.OnStopFn()
	}
}

type MatcherStep struct {
	Matcher    func(JsonLogRecord) bool
	OnMatch    func(JsonLogRecord) error
	NotMatcher func(JsonLogRecord) bool
	OnNotMatch func(JsonLogRecord) error
}

func LineMatcher(matchFn func(JsonLogRecord) bool, onMatch ...func(JsonLogRecord) error) *MatcherStep {
	res := &MatcherStep{
		Matcher: matchFn,
	}
	if len(onMatch) > 0 {
		res.OnMatch = onMatch[0]
	}
	return res
}

func AwaitMatch(matchFn func(JsonLogRecord) bool, onMatch ...func(JsonLogRecord) error) *MatcherStep {
	return LineMatcher(matchFn, onMatch...)
}

func LineNotMatcher(matchFn func(JsonLogRecord) bool, onMatch ...func(JsonLogRecord) error) *MatcherStep {
	res := &MatcherStep{
		NotMatcher: matchFn,
	}
	if len(onMatch) > 0 {
		res.OnNotMatch = onMatch[0]
	} else {
		res.OnNotMatch = func(r JsonLogRecord) error {
			return fmt.Errorf("log line matches negative matcher: %+v", r)
		}
	}
	return res
}

func AwaitNotMatch(notMatchFn func(JsonLogRecord) bool, onMatch ...func(JsonLogRecord) error) *MatcherStep {
	return LineNotMatcher(notMatchFn, onMatch...)
}

type JsonLogMatcher struct {
	Steps    []*MatcherStep
	Index    int
	finished bool
	err      error
}

func NewJsonLogMatcher(steps ...*MatcherStep) *JsonLogMatcher {
	return &JsonLogMatcher{
		Steps:    steps,
		Index:    0,
		finished: false,
	}
}

func (m *JsonLogMatcher) HandleRecord(_ JsonLogRecord) {
}

func (m *JsonLogMatcher) HandleLine(line string) {
	// fmt.Printf("Matcher Got line: %s\n", line)
	// defer func() {
	//	fmt.Printf("matcher.HandleLine done: ind=%d fin=%v err=%v\n", m.Index, m.finished, m.err)
	// }()
	defer func() {
		e := recover()
		if e != nil {
			m.finished = true
		}
	}()

	step := m.Steps[m.Index]

	// NotMatcher step should always handle line. Matcher step should not handle line after finish.
	if m.finished && step.Matcher != nil {
		return
	}
	// Ω(line).Should(HavePrefix("{"))

	logRecord, _ := NewJsonLogRecord().FromString(line)
	// Ω(err).ShouldNot(HaveOccurred())

	var err error
	if step.Matcher != nil {
		res := step.Matcher(logRecord)
		if res {
			if step.OnMatch != nil {
				// fmt.Printf("matcher.HandleLine OnMatch()\n")
				err = step.OnMatch(logRecord)
				// fmt.Printf("matcher.HandleLine OnMatchDone()\n")
				if err != nil {
					m.err = err
					m.finished = true
					return
				}
			}
			if m.Index == len(m.Steps)-1 {
				m.finished = true
			} else {
				m.Index++
			}
		}
	}
	if step.NotMatcher != nil {
		// NotMatcher is a non-blocking step, it should be always considered as finished
		m.finished = true
		res := step.NotMatcher(logRecord)
		if res {
			if step.OnNotMatch != nil {
				err := step.OnNotMatch(logRecord)
				if err != nil {
					m.err = err
					return
				}
			}
			if m.Index == len(m.Steps)-1 {
				m.finished = true
			} else {
				m.Index++
			}
		}
	}
}

func (m *JsonLogMatcher) Finished() bool {
	return m.finished
}

func (m *JsonLogMatcher) Error() error {
	return m.err
}

func (m *JsonLogMatcher) Reset() {
	m.Index = 0
	m.finished = false
	m.err = nil
}

func FinishAllMatchersSuccessfully() types.GomegaMatcher {
	return &FinishAllMatchersSuccessfullyMatcher{}
}

type FinishAllMatchersSuccessfullyMatcher struct {
	finished bool
	err      error
}

func (matcher *FinishAllMatchersSuccessfullyMatcher) Match(actual interface{}) (success bool, err error) {
	analyzer, ok := actual.(*JsonLogAnalyzer)
	if !ok {
		return false, fmt.Errorf("FinishAllMatchersSuccessfully must be passed a JsonLogAnalyzer. Got %T\n", actual)
	}

	matcher.err = analyzer.Error()
	matcher.finished = analyzer.Finished()

	return matcher.finished && matcher.err == nil, nil
}

func (matcher *FinishAllMatchersSuccessfullyMatcher) FailureMessage(_ interface{}) (message string) {
	msgs := []string{}
	if !matcher.finished {
		msgs = append(msgs, "is finished")
	}
	if matcher.err != nil {
		msgs = append(msgs, fmt.Sprintf("has no error. Got %v", matcher.err))
	}
	return fmt.Sprintf("Expected JsonLogAnalyzer %s", strings.Join(msgs, " and "))
}

func (matcher *FinishAllMatchersSuccessfullyMatcher) NegatedFailureMessage(_ interface{}) (message string) {
	msgs := []string{}
	if matcher.finished {
		msgs = append(msgs, "is not finished")
	}
	if matcher.err == nil {
		msgs = append(msgs, "has error")
	}
	return fmt.Sprintf("Expected JsonLogAnalyzer %s", strings.Join(msgs, " or "))
}
