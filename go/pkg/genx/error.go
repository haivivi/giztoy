package genx

import (
	"errors"
	"fmt"
)

// ErrDone is returned when the stream is done.
var ErrDone = errors.New("genx: done")

func Done(stats Usage) *State {
	return &State{
		usage:  stats,
		status: StatusDone,
		err:    ErrDone,
	}
}

func Blocked(stats Usage, refusal string) *State {
	return &State{
		usage:  stats,
		status: StatusBlocked,
		err:    fmt.Errorf("genx: generate blocked: %s", refusal),
	}
}

func Truncated(stats Usage) *State {
	return &State{
		usage:  stats,
		status: StatusTruncated,
		err:    errors.New("genx: generate truncated"),
	}
}

func Error(stats Usage, err error) *State {
	return &State{
		usage:  stats,
		status: StatusError,
		err:    fmt.Errorf("genx: generate error: %w", err),
	}
}

type State struct {
	usage  Usage
	status Status
	err    error
}

func (ss State) Usage() Usage {
	return ss.usage
}

func (ss State) Status() Status {
	return ss.status
}

func (ss State) Unwrap() error {
	return ss.err
}

func (ss State) Error() string {
	switch ss.status {
	case StatusDone:
		return "genx: generate done"
	case StatusTruncated:
		return ss.err.Error()
	case StatusBlocked:
		return ss.err.Error()
	case StatusError:
		return ss.err.Error()
	default:
		return fmt.Sprintf("genx: unexpected stream status: %v", ss.status)
	}
}
