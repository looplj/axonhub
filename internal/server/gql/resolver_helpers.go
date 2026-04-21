package gql

import (
	"fmt"
	"time"
)

// copyTimePtr creates a deep copy of a *time.Time, normalizing to UTC.
func copyTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	cp := t.UTC()
	return &cp
}

func validateWindowAnchor(anchor *time.Time, fieldName string) error {
	if anchor == nil {
		return nil
	}
	*anchor = anchor.UTC()
	if anchor.IsZero() {
		return fmt.Errorf("%s must not be a zero time", fieldName)
	}
	return nil
}
