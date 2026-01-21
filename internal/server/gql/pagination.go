package gql

import "fmt"

const maxPaginationLimit = 1000

// validatePaginationArgs ensures GraphQL list queries receive a bounded window.
func validatePaginationArgs(first, last *int) error {
	provided := false

	if first != nil {
		provided = true

		if *first <= 0 {
			return fmt.Errorf("first must be greater than 0")
		}

		if *first > maxPaginationLimit {
			return fmt.Errorf("first cannot exceed %d", maxPaginationLimit)
		}
	}

	if last != nil {
		provided = true

		if *last <= 0 {
			return fmt.Errorf("last must be greater than 0")
		}

		if *last > maxPaginationLimit {
			return fmt.Errorf("last cannot exceed %d", maxPaginationLimit)
		}
	}

	if !provided {
		return fmt.Errorf("either first or last must be provided")
	}

	return nil
}
