package shared

import (
	"bytes"
	"encoding/base64"
	"strings"
)

// SignatureProvider identifies which AI provider generated a signature/reasoning blob.
type SignatureProvider string

const (
	ProviderAnthropic SignatureProvider = "anthropic"
	ProviderGemini    SignatureProvider = "gemini"
	ProviderOpenAI    SignatureProvider = "openai"
	ProviderUnknown   SignatureProvider = "unknown"
)

// GuessResult is the result of guessing a signature's provider.
type GuessResult struct {
	Provider SignatureProvider
	Reasons  []string
}

// GuessSignatureProvider guesses which AI provider a raw base64 blob belongs to.
//
// Heuristics:
//   - "gAAAA*" / "gAAA*" prefix → OpenAI
//   - standard base64 whose decoded bytes contain an Anthropic model marker
//     (for example, "claude-sonnet-5") → Anthropic
//   - standard base64 with protobuf-like decoded bytes → Gemini
//   - otherwise → unknown
func GuessSignatureProvider(raw string) GuessResult {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, `"`)
	s = strings.TrimSpace(s)

	reason := make([]string, 0, 3)

	// OpenAI encrypted_content common pattern: gAAAA... (base64url Fernet/fernet-like)
	if strings.HasPrefix(s, "gAAAA") || strings.HasPrefix(s, "gAAA") {
		reason = append(reason, "starts with gAAA*, commonly seen in OpenAI reasoning.encrypted_content")
		return GuessResult{
			Provider: ProviderOpenAI,
			Reasons:  reason,
		}
	}

	isStdBase64 := isStdBase64String(s)

	if isStdBase64 {
		bytes, err := decodeStdBase64(s)
		if err == nil && containsAnthropicModel(bytes) {
			reason = append(reason, "standard base64 decoded bytes contain an Anthropic model marker")
			return GuessResult{
				Provider: ProviderAnthropic,
				Reasons:  reason,
			}
		}

		// Check standard base64 for protobuf-like bytes (Gemini).
		// Valid standard base64 is never OpenAI — OpenAI uses base64url encoding.
		if err == nil && looksLikeProto(bytes) {
			reason = append(reason, "standard base64 and decoded bytes look protobuf-like, which fits Gemini thoughtSignature")
			return GuessResult{
				Provider: ProviderGemini,
				Reasons:  reason,
			}
		}

		reason = append(reason, "standard base64 without known provider prefix")
		return GuessResult{
			Provider: ProviderUnknown,
			Reasons:  reason,
		}
	}

	reason = append(reason, "not a recognized base64/base64url thinking signature shape")
	return GuessResult{
		Provider: ProviderUnknown,
		Reasons:  reason,
	}
}

// decodeStdBase64 accepts both padded and unpadded standard Base64. Anthropic
// signatures are opaque bytes and commonly have a length divisible by three,
// but accepting the unpadded form makes detection safe across proxies/clients.
func decodeStdBase64(s string) ([]byte, error) {
	if strings.ContainsRune(s, '=') {
		return base64.StdEncoding.DecodeString(s)
	}
	return base64.RawStdEncoding.DecodeString(s)
}

// containsAnthropicModel checks for provider-specific model identifiers in the
// decoded opaque payload. Signatures are binary (often protobuf-like), so the
// model name is searched as an ASCII substring rather than treating the whole
// payload as text or attempting to fully parse its wire format.
func containsAnthropicModel(buf []byte) bool {
	lower := bytes.ToLower(buf)
	return bytes.Contains(lower, []byte("claude")) || bytes.Contains(lower, []byte("anthropic"))
}

// isStdBase64String checks if s matches standard base64 pattern [A-Za-z0-9+/]+={0,2}.
// Padding '=' must only appear at the end, with at most 2 padding characters.
func isStdBase64String(s string) bool {
	if s == "" {
		return false
	}
	paddingStarted := false
	paddingCount := 0
	for _, c := range s {
		switch {
		case c >= 'A' && c <= 'Z',
			c >= 'a' && c <= 'z',
			c >= '0' && c <= '9',
			c == '+' || c == '/':
			if paddingStarted {
				// non-padding character after padding started
				return false
			}
		case c == '=':
			paddingStarted = true
			paddingCount++
			if paddingCount > 2 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// looksLikeProto checks if the bytes look like a valid protobuf message.
// Traverses as many fields as possible to build confidence, rejecting
// messages with invalid wire types, field number 0, or length-delimited
// fields whose varint length exceeds buffer bounds.
func looksLikeProto(buf []byte) bool {
	if len(buf) == 0 {
		return false
	}

	offset := 0

	for offset < len(buf) {
		// Each field starts with a varint tag (key)
		tag, n := readVarint64(buf[offset:])
		if n == 0 {
			return offset > 0
		}
		offset += n

		wireType := int(tag & 0x07)
		fieldNum := tag >> 3

		if fieldNum == 0 {
			return false
		}

		// Reject deprecated group wire types (3=start group, 4=end group)
		if wireType == 3 || wireType == 4 {
			return false
		}

		switch wireType {
		case 0: // varint
			_, n = readVarint64(buf[offset:])
			if n == 0 {
				return false
			}
			offset += n
		case 1: // 64-bit (8 bytes, little-endian)
			if offset+8 > len(buf) {
				return false
			}
			offset += 8
		case 2: // length-delimited
			length, n := readVarint64(buf[offset:])
			if n == 0 || length > uint64(len(buf)-offset-n) { //nolint:gosec // G115 - length bounded by remaining buffer
				return false
			}
			offset += n + int(length) //nolint:gosec // G115 - length verified above
		case 5: // 32-bit (4 bytes, little-endian)
			if offset+4 > len(buf) {
				return false
			}
			offset += 4
		default:
			return false
		}
	}

	return true
}

// readVarint64 decodes a protobuf varint from buf, returning the value
// and the number of bytes consumed. Returns (0, 0) on invalid input.
// Supports up to 10 bytes (full 64-bit protobuf varint).
func readVarint64(buf []byte) (uint64, int) {
	var result uint64
	for i := 0; i < len(buf) && i < 10; i++ {
		result |= uint64(buf[i]&0x7F) << (i * 7)
		if buf[i]&0x80 == 0 {
			return result, i + 1
		}
	}
	return 0, 0
}
