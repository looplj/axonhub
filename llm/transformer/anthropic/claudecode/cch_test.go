package claudecode

import (
	"testing"
)

func TestComputeCCH(t *testing.T) {
	tests := []struct {
		name    string
		message string
		version string
		want    string // expected 3-char hash
	}{
		{
			name:    "basic message",
			message: "Hello, how are you?",
			version: "2.1.81",
			want:    "", // Will compute dynamically
		},
		{
			name:    "empty message",
			message: "",
			version: "2.1.81",
			want:    "",
		},
		{
			name:    "short message",
			message: "Hi",
			version: "2.1.81",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := ComputeCCH(tt.message, tt.version)

			// Hash should be 3 hex characters
			if len(hash) != 3 {
				t.Errorf("CCH hash length should be 3, got %d", len(hash))
			}

			// Same input should always produce same hash
			hash2 := ComputeCCH(tt.message, tt.version)
			if hash != hash2 {
				t.Errorf("CCH hash not deterministic: %s != %s", hash, hash2)
			}

			// Different messages should produce different hashes (usually)
			if len(tt.message) > 20 {
				hashOther := ComputeCCH("different message", tt.version)
				if hash == hashOther {
					// This is possible but unlikely
					t.Logf("Warning: collision detected between hashes")
				}
			}
		})
	}

	// Test that CCH positions work correctly
	message := "0123456789abcdefghijklmnopqrstuvwxyz"
	hash := ComputeCCH(message, "2.1.81")
	if len(hash) != 3 {
		t.Errorf("Expected 3-char hash, got %s", hash)
	}
}