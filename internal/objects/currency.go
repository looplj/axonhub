package objects

import (
	"fmt"
	"strings"

	"golang.org/x/text/currency"
)

// DefaultCurrencyCode is the currency used by the model library and by records
// created before multi-currency support existed.
const DefaultCurrencyCode = "USD"

// NormalizeCurrencyCode trims and upper-cases an ISO 4217 currency code,
// falling back to DefaultCurrencyCode when the input is empty.
func NormalizeCurrencyCode(code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return DefaultCurrencyCode, nil
	}

	if _, err := currency.ParseISO(code); err != nil {
		return "", fmt.Errorf("invalid currency code %q: %w", code, err)
	}

	return code, nil
}
