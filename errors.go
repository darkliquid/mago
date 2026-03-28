//go:build linux

package mago

import "fmt"

type OpError struct {
	Op          string
	Code        Result
	Description string
}

type VersionMismatchError struct {
	Expected       Version
	Actual         Version
	ExpectedString string
	ActualString   string
}

func (e *OpError) Error() string {
	switch {
	case e == nil:
		return "<nil>"
	case e.Description != "":
		return fmt.Sprintf("%s failed: %s (%d)", e.Op, e.Description, e.Code)
	case e.Op != "":
		return fmt.Sprintf("%s failed with result %d", e.Op, e.Code)
	default:
		return fmt.Sprintf("miniaudio result %d", e.Code)
	}
}

func (e *VersionMismatchError) Error() string {
	if e == nil {
		return "<nil>"
	}

	return fmt.Sprintf(
		"mago: loaded miniaudio version mismatch: expected %s (%s), got %s (%s)",
		e.Expected.String(),
		e.ExpectedString,
		e.Actual.String(),
		e.ActualString,
	)
}
