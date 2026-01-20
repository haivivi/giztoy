package mqtt

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	// ErrInvalidShareSubscription is returned when a share subscription is
	// invalid.
	ErrInvalidShareSubscription = errors.New("invalid share subscription")

	// ErrInvalidQueueSubscription is returned when a queue subscription is
	// invalid.
	ErrInvalidQueueSubscription = errors.New("invalid queue subscription")

	// ErrInvalidTopicPattern is returned when a topic pattern is invalid.
	ErrInvalidTopicPattern = errors.New("invalid topic pattern")
)

type trie struct {
	children map[string]*trie
	matchAny *trie
	matchAll *trie

	handlers []Handler
}

func (t *trie) Set(pattern string, f func(t *trie)) error {
	if len(pattern) == 0 {
		f(t)
		return nil
	}

	var first, subseq string
	idx := strings.Index(pattern, "/")
	switch idx {
	case -1:
		first = pattern
	default:
		first = pattern[:idx]
		switch first {
		default:
			subseq = pattern[idx+1:]
		case "$share": // $share/<sharename/first/subseq>
			ss := strings.SplitN(pattern[idx+1:], "/", 3)
			if len(ss) < 2 {
				return ErrInvalidShareSubscription
			}
			first = ss[1]
			if len(ss) == 3 {
				subseq = ss[2]
			}
		case "$queue": // $queue/<first/subseq>
			ss := strings.SplitN(pattern[idx+1:], "/", 2)
			if len(ss) < 1 {
				return ErrInvalidQueueSubscription
			}
			first = ss[0]
			if len(ss) == 2 {
				subseq = ss[1]
			}
		}
	}

	if t.children != nil {
		if ch, ok := t.children[first]; ok {
			return ch.Set(subseq, f)
		}
	}

	switch first {

	case "+": // .../<first:+>/<subseq>
		if t.matchAny == nil {
			t.matchAny = &trie{}
		}
		return t.matchAny.Set(subseq, f)
	case "#": // .../<first:#>
		if len(subseq) != 0 {
			return ErrInvalidTopicPattern
		}
		if t.matchAll == nil {
			t.matchAll = &trie{}
		}
		f(t.matchAll)
		return nil
	default: // .../<first>/<subseq>
		if t.children == nil {
			t.children = make(map[string]*trie)
		}
		ch := &trie{}
		t.children[first] = ch
		return ch.Set(subseq, f)
	}
}

func (t *trie) Get(topic string) ([]Handler, bool) {
	_, handlers, ok := t.match("", topic)
	return handlers, ok
}

func (t *trie) match(matched, topic string) (string, []Handler, bool) {
	if len(topic) == 0 {
		return matched, t.handlers, len(t.handlers) > 0
	}
	var first, subseq string
	p := strings.IndexByte(topic, '/')
	if p == -1 {
		first = topic
	} else {
		first = topic[:p]
		subseq = topic[p+1:]
	}
	if t.children != nil {
		if ch, ok := t.children[first]; ok {
			if route, handlers, ok := ch.match(matched+"/"+first, subseq); ok {
				return route, handlers, true
			}
		}
	}
	if t.matchAny != nil {
		if route, handlers, ok := t.matchAny.match(matched+"/+", subseq); ok {
			return route, handlers, true
		}
	}
	if t.matchAll != nil {
		if route, handlers, ok := t.matchAll.match(matched+"/#", ""); ok {
			return route, handlers, true
		}
	}
	return "", nil, false
}

func (t *trie) walkWithPath(path []string, f func([]string, *trie)) {
	if t.children != nil {
		for seg, ch := range t.children {
			ch.walkWithPath(append(path, seg), f)
		}
	}
	if t.matchAny != nil {
		t.matchAny.walkWithPath(append(path, "+"), f)
	}
	if t.matchAll != nil {
		t.matchAll.walkWithPath(append(path, "#"), f)
	}
	f(path, t)
}

func (t *trie) String() string {
	var lines []string
	t.walkWithPath(nil, func(path []string, node *trie) {
		lines = append(lines, fmt.Sprintf("%p: %s: %d", node, strings.Join(path, "/"), len(node.handlers)))
	})
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}
