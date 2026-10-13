package objects

import (
	"bytes"
	"testing"
)

func TestChannelResponseTimeoutMode_MarshalGQL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode ChannelResponseTimeoutMode
		want string
	}{
		{name: "zero value is inherit", mode: ChannelResponseTimeoutModeInherit, want: `"INHERIT"`},
		{name: "custom", mode: ChannelResponseTimeoutModeCustom, want: `"CUSTOM"`},
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

func TestChannelResponseTimeoutMode_UnmarshalGQL(t *testing.T) {
	t.Parallel()

	// The GraphQL input declares mode as nullable, so a client may send an
	// explicit null. It has to mean the zero value (inherit) instead of being
	// rejected, matching APIKeyAutoDisableMode.
	t.Run("null is inherit", func(t *testing.T) {
		t.Parallel()

		mode := ChannelResponseTimeoutModeCustom
		if err := mode.UnmarshalGQL(nil); err != nil {
			t.Fatalf("UnmarshalGQL(nil) error = %v", err)
		}
		if mode != ChannelResponseTimeoutModeInherit {
			t.Fatalf("UnmarshalGQL(nil) = %q, want inherit", mode)
		}
	})

	t.Run("INHERIT", func(t *testing.T) {
		t.Parallel()

		var mode ChannelResponseTimeoutMode
		if err := mode.UnmarshalGQL("INHERIT"); err != nil {
			t.Fatalf("UnmarshalGQL(INHERIT) error = %v", err)
		}
		if mode != ChannelResponseTimeoutModeInherit {
			t.Fatalf("UnmarshalGQL(INHERIT) = %q, want inherit", mode)
		}
	})

	t.Run("CUSTOM", func(t *testing.T) {
		t.Parallel()

		var mode ChannelResponseTimeoutMode
		if err := mode.UnmarshalGQL("CUSTOM"); err != nil {
			t.Fatalf("UnmarshalGQL(CUSTOM) error = %v", err)
		}
		if mode != ChannelResponseTimeoutModeCustom {
			t.Fatalf("UnmarshalGQL(CUSTOM) = %q, want custom", mode)
		}
	})

	t.Run("rejects non-string", func(t *testing.T) {
		t.Parallel()

		var mode ChannelResponseTimeoutMode
		if err := mode.UnmarshalGQL(1); err == nil {
			t.Fatal("UnmarshalGQL(1) want error")
		}
	})

	t.Run("rejects unknown value", func(t *testing.T) {
		t.Parallel()

		var mode ChannelResponseTimeoutMode
		if err := mode.UnmarshalGQL("SOMETHING"); err == nil {
			t.Fatal("UnmarshalGQL(SOMETHING) want error")
		}
	})
}
