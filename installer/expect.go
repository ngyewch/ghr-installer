package installer

import (
	"strings"
)

type Expecter struct {
	valid   bool
	s       string
	matched string
}

func NewExpecter(s string) *Expecter {
	return &Expecter{
		valid: true,
		s:     s,
	}
}

func (expecter *Expecter) Matched() string {
	if !expecter.valid {
		return ""
	}
	return expecter.matched
}

func (expecter *Expecter) ExpectString(s string) bool {
	if !expecter.valid {
		return false
	}
	if !strings.HasPrefix(expecter.s, s) {
		expecter.valid = false
		return false
	}
	expecter.matched += s
	expecter.s = expecter.s[len(s):]
	return true
}

func (expecter *Expecter) ExpectStrings(ss []string) bool {
	if !expecter.valid {
		return false
	}
	for _, s := range ss {
		if strings.HasPrefix(expecter.s, s) {
			expecter.matched += s
			expecter.s = expecter.s[len(s):]
			return true
		}
	}
	expecter.valid = false
	return false
}

func (expecter *Expecter) PeekString(s string) bool {
	if !expecter.valid {
		return false
	}
	return strings.HasPrefix(expecter.s, s)
}

func (expecter *Expecter) PeekStrings(ss []string) bool {
	if !expecter.valid {
		return false
	}
	for _, s := range ss {
		if strings.HasPrefix(expecter.s, s) {
			return true
		}
	}
	return false
}

func (expecter *Expecter) IsEmpty() bool {
	return len(expecter.s) == 0
}
