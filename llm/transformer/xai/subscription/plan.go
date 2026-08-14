package subscription

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
)

// PlanFromAccessToken reads the provider-issued tier claim for display only.
// Authorization continues to rely on the token endpoint, not this unsigned payload.
func PlanFromAccessToken(accessToken string) string {
	parts := strings.Split(accessToken, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Tier json.RawMessage `json:"tier"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || len(claims.Tier) == 0 {
		return ""
	}
	var number json.Number
	if err := json.Unmarshal(claims.Tier, &number); err == nil {
		if tier, err := strconv.ParseUint(number.String(), 10, 64); err == nil {
			return planFromTierNumber(tier)
		}
	}
	var text string
	if err := json.Unmarshal(claims.Tier, &text); err != nil {
		return ""
	}
	if tier, err := strconv.ParseUint(strings.TrimSpace(text), 10, 64); err == nil {
		return planFromTierNumber(tier)
	}
	return planFromTierName(text)
}

func planFromTierNumber(tier uint64) string {
	switch tier {
	case 0:
		return "Free"
	case 1:
		return "SuperGrok"
	case 2:
		return "X Basic"
	case 3:
		return "X Premium"
	case 4:
		return "X Premium Plus"
	case 5:
		return "SuperGrok Heavy"
	case 6:
		return "SuperGrok Lite"
	case 7:
		return "SuperGrok Plus"
	default:
		return ""
	}
}

func planFromTierName(raw string) string {
	tier := strings.ToLower(strings.TrimSpace(raw))
	tier = strings.ReplaceAll(tier, "-", "_")
	tier = strings.Join(strings.Fields(tier), "_")
	switch tier {
	case "free", "grok_free", "grokfree", "free_tier", "freetier", "grok_basic", "grokbasic":
		return "Free"
	case "supergrok", "grokpro", "supergrok_pro", "supergrokpro", "paid", "pro":
		return "SuperGrok"
	case "supergrok_lite", "supergroklite":
		return "SuperGrok Lite"
	case "supergrok_heavy", "supergrokheavy":
		return "SuperGrok Heavy"
	case "supergrok_plus", "supergrokplus":
		return "SuperGrok Plus"
	case "x_basic", "xbasic", "basic":
		return "X Basic"
	case "x_premium", "xpremium":
		return "X Premium"
	case "x_premium_plus", "xpremiumplus", "x_premium+":
		return "X Premium Plus"
	default:
		return ""
	}
}
