package objects

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeCurrencyCode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty defaults to USD", input: "", want: DefaultCurrencyCode},
		{name: "trims and upper cases", input: " cny ", want: "CNY"},
		{name: "valid ISO code", input: "eur", want: "EUR"},
		{name: "invalid code", input: "NOT_A_CURRENCY", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeCurrencyCode(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
