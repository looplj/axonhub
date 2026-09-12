package objects

import (
	"bytes"
	"testing"
)

func TestAPIKeyAutoDisableMode_MarshalGQL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode APIKeyAutoDisableMode
		want string
	}{
		{name: "empty is null", mode: "", want: "null"},
		{name: "inherit", mode: APIKeyAutoDisableModeInherit, want: `"inherit"`},
		{name: "custom", mode: APIKeyAutoDisableModeCustom, want: `"custom"`},
		{name: "off", mode: APIKeyAutoDisableModeOff, want: `"off"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			tt.mode.MarshalGQL(&buf)
			if got := buf.String(); got != tt.want {
				t.Fatalf("MarshalGQL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAPIKeyAutoDisableMode_UnmarshalGQL(t *testing.T) {
	t.Parallel()

	t.Run("null is unset", func(t *testing.T) {
		t.Parallel()
		var mode APIKeyAutoDisableMode
		if err := mode.UnmarshalGQL(nil); err != nil {
			t.Fatalf("UnmarshalGQL(nil) error = %v", err)
		}
		if mode != "" {
			t.Fatalf("UnmarshalGQL(nil) = %q, want empty", mode)
		}
	})

	t.Run("inherit", func(t *testing.T) {
		t.Parallel()
		var mode APIKeyAutoDisableMode
		if err := mode.UnmarshalGQL("inherit"); err != nil {
			t.Fatalf("UnmarshalGQL(inherit) error = %v", err)
		}
		if mode != APIKeyAutoDisableModeInherit {
			t.Fatalf("UnmarshalGQL(inherit) = %q, want inherit", mode)
		}
	})

	t.Run("rejects non-string", func(t *testing.T) {
		t.Parallel()
		var mode APIKeyAutoDisableMode
		if err := mode.UnmarshalGQL(1); err == nil {
			t.Fatal("UnmarshalGQL(1) want error")
		}
	})
}
