// +build test

package utils

import (
	"fmt"
)

type JsonLogAnalyzer struct {
	Matchers [][]*JsonLogMatcher
	OnStopFn func()
	Index    int
	finished bool
	err      error
}

func NewJsonLogAnalyzer() *JsonLogAnalyzer {
	return &JsonLogAnalyzer{
		Matchers: make([][]*JsonLogMatcher, 0),
		Index:    0,
		finished: false,
	}
}

func (r *JsonLogAnalyzer) Add(analyzers ...*JsonLogMatcher) {
	r.Matchers = append(r.Matchers, analyzers)
}

func (r *JsonLogAnalyzer) OnStop(fn func()) {
	r.OnStopFn = fn
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
	a.finished = false
	a.err = nil
}

func (a *JsonLogAnalyzer) HandleLine(line string) {
	fmt.Printf("Got line: %s\n", line)
	defer func() {
		fmt.Printf("analyzer.HandleLine done: ind=%d fin=%v err=%v\n", a.Index, a.finished, a.err)
	}()
	if a.finished {
		return
	}

	matchers := a.Matchers[a.Index]

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
		}
		if a.Index == len(a.Matchers) {
			a.finished = true
		}
	}

	if a.finished && a.OnStopFn != nil {
		fmt.Printf("analyzer.HandleLine OnStopFn()\n")
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

func ShouldMatch(matchFn func(JsonLogRecord) bool, onMatch ...func(JsonLogRecord) error) *MatcherStep {
	return LineMatcher(matchFn, onMatch...)
}

func ThenShouldBeLine(matchFn func(JsonLogRecord) bool, onMatch ...func(JsonLogRecord) error) *MatcherStep {
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

func ShouldNotMatch(notMatchFn func(JsonLogRecord) bool, onMatch ...func(JsonLogRecord) error) *MatcherStep {
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

func (m *JsonLogMatcher) HandleRecord(r JsonLogRecord) {

}

func (m *JsonLogMatcher) HandleLine(line string) {
	//fmt.Printf("Matcher Got line: %s\n", line)
	defer func() {
		fmt.Printf("matcher.HandleLine done: ind=%d fin=%v err=%v\n", m.Index, m.finished, m.err)
	}()
	defer func() {
		if m.Index == len(m.Steps) {
			m.finished = true
		}
	}()
	defer func() {
		e := recover()
		if e != nil {
			m.finished = true
		}
	}()

	if m.finished {
		return
	}
	//Ω(line).Should(HavePrefix("{"))

	logRecord, err := NewJsonLogRecord().FromString(line)
	//Ω(err).ShouldNot(HaveOccurred())

	step := m.Steps[m.Index]

	if step.Matcher != nil {
		res := step.Matcher(logRecord)
		if res {
			if step.OnMatch != nil {
				fmt.Printf("matcher.HandleLine OnMatch()\n")
				err = step.OnMatch(logRecord)
				fmt.Printf("matcher.HandleLine OnMatchDone()\n")
				if err != nil {
					m.err = err
					m.finished = true
					return
				}
			}
			m.Index++
		}
	} else {
		if step.NotMatcher != nil {
			res := step.NotMatcher(logRecord)
			if res {
				if step.OnNotMatch != nil {
					err := step.OnNotMatch(logRecord)
					if err != nil {
						m.err = err
						m.finished = true
						return
					}
				}
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
