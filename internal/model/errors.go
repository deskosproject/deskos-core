package model

import (
	"fmt"
	"sort"
	"strings"
)

// ErrorList accumulates independent errors so a single run can report all
// of them. Rendering is sorted so output is deterministic.
type ErrorList []error

// Add appends err, flattening a direct ErrorList. A wrapped list keeps its
// wrapper: a caller that prefixes a workstation or resource name must not lose
// it when the inner list is flattened.
func (l *ErrorList) Add(err error) {
	if err == nil {
		return
	}
	if nested, ok := err.(ErrorList); ok {
		*l = append(*l, nested...)
		return
	}
	*l = append(*l, err)
}

// Err returns nil when empty, otherwise the list itself.
func (l ErrorList) Err() error {
	if len(l) == 0 {
		return nil
	}
	return l
}

func (l ErrorList) Error() string {
	msgs := make([]string, 0, len(l))
	for _, e := range l {
		msgs = append(msgs, e.Error())
	}
	sort.Strings(msgs)
	return strings.Join(msgs, "\n\n")
}

// Messages returns the sorted individual messages.
func (l ErrorList) Messages() []string {
	msgs := make([]string, 0, len(l))
	for _, e := range l {
		msgs = append(msgs, e.Error())
	}
	sort.Strings(msgs)
	return msgs
}

// Errorf formats an error attributed to a resource.
func Errorf(res *Resource, format string, a ...any) error {
	return fmt.Errorf("%s: %s: %s", res.Source, res.ID(), fmt.Sprintf(format, a...))
}
