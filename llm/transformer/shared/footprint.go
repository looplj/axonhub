package shared

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

var footprintCache sync.Map

func computeFootprint(baseURL, salt string) string {
	h := sha256.Sum256([]byte(baseURL + salt))
	return hex.EncodeToString(h[:3])
}

func ComputeFootprint(baseURL, accountIdentity string) string {
	if baseURL == "" || accountIdentity == "" {
		return ""
	}
	key := baseURL + "\x00" + accountIdentity
	if v, ok := footprintCache.Load(key); ok {
		return v.(string)
	}
	fp := computeFootprint(baseURL, accountIdentity)
	footprintCache.Store(key, fp)
	return fp
}

func isFootprintHex6(s string) bool {
	if len(s) != 6 {
		return false
	}

	for i := range len(s) {
		c := s[i]
		if c >= '0' && c <= '9' {
			continue
		}
		if c >= 'a' && c <= 'f' {
			continue
		}
		return false
	}

	return true
}
