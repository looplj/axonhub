package claudecode

import (
	"testing"
)

func TestGenerateIdentityFromAccount(t *testing.T) {
	tests := []struct {
		name            string
		accountIdentity string
	}{
		{"account 1", "1"},
		{"account 100", "100"},
		{"account 999", "999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity1 := GenerateIdentityFromAccount(tt.accountIdentity)
			identity2 := GenerateIdentityFromAccount(tt.accountIdentity)

			// Same account identity should always generate the same identity
			if identity1.DeviceID != identity2.DeviceID {
				t.Errorf("DeviceID not deterministic: %s != %s", identity1.DeviceID, identity2.DeviceID)
			}

			if identity1.AccountUUID != identity2.AccountUUID {
				t.Errorf("AccountUUID not deterministic: %s != %s", identity1.AccountUUID, identity2.AccountUUID)
			}

			// DeviceID should be 64 hex characters
			if len(identity1.DeviceID) != 64 {
				t.Errorf("DeviceID length should be 64, got %d", len(identity1.DeviceID))
			}

			// AccountUUID should be valid UUID format
			if len(identity1.AccountUUID) != 36 {
				t.Errorf("AccountUUID length should be 36, got %d", len(identity1.AccountUUID))
			}

			// Different account identities should generate different identities
			if tt.accountIdentity == "1" {
				identityOther := GenerateIdentityFromAccount("2")
				if identity1.DeviceID == identityOther.DeviceID {
					t.Error("Different account identities should generate different DeviceIDs")
				}
			}
		})
	}
}