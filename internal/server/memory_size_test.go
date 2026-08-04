package server

import (
	"testing"
)

func TestMemorySizeUnmarshalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    MemorySize
		wantErr bool
	}{
		{name: "empty", input: "", want: 0},
		{name: "plain bytes", input: "536870912", want: 536870912},
		{name: "plain zero", input: "0", want: 0},
		{name: "plain negative", input: "-1", wantErr: true},
		{name: "K upper", input: "64K", want: 64 << 10},
		{name: "M upper", input: "512M", want: 512 << 20},
		{name: "G upper", input: "1G", want: 1 << 30},
		{name: "MB upper", input: "512MB", want: 512 << 20},
		{name: "mb lower", input: "512mb", want: 512 << 20},
		{name: "m lower", input: "32m", want: 32 << 20},
		{name: "gb mixed", input: "1.5Gb", want: MemorySize(int64(1.5 * float64(1<<30)))},
		{name: "spaces", input: "  64K  ", want: 64 << 10},
		{name: "nan", input: "NaNM", wantErr: true},
		{name: "inf", input: "InfG", wantErr: true},
		{name: "negative float", input: "-1.5M", wantErr: true},
		{name: "overflow", input: "999999999999G", wantErr: true},
		{name: "unknown suffix", input: "10T", wantErr: true},
		{name: "invalid number", input: "abcM", wantErr: true},
		{name: "too short", input: "M", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got MemorySize
			err := got.UnmarshalText([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("input %q: expected error, got value %d", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("input %q: unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("input %q: got %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestMemorySizeRejectsScaledOverflow(t *testing.T) {
	t.Parallel()

	// 2^33 G scales to exactly 2^63, the first value not representable in int64.
	// float64(math.MaxInt64) rounds to 2^63, so the guard must use >= to reject it.
	const input = "8589934592G"

	var got MemorySize
	if err := got.UnmarshalText([]byte(input)); err == nil {
		t.Fatalf("expected overflow error for %q, got %d", input, got)
	}
}
