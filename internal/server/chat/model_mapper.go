package chat

import (
	"context"
	"regexp"
	"strings"
	"sync"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

// patternCache holds compiled regex patterns and exact match flags.
type patternCache struct {
	regex      *regexp.Regexp
	exactMatch bool
	compileErr bool
}

// ModelMapper handles model mapping based on API key profiles.
type ModelMapper struct {
	mu    sync.RWMutex
	cache map[string]*patternCache
}

// NewModelMapper creates a new ModelMapper instance.
func NewModelMapper() *ModelMapper {
	return &ModelMapper{
		cache: make(map[string]*patternCache),
	}
}

// MapModel applies model mapping from API key profiles if an active profile exists
// Returns the mapped model name or the original model if no mapping is found.
func (m *ModelMapper) MapModel(ctx context.Context, apiKey *ent.APIKey, originalModel string) string {
	if apiKey == nil || apiKey.Profiles == nil {
		return originalModel
	}

	profiles := apiKey.Profiles
	if profiles.ActiveProfile == "" {
		log.Debug(ctx, "No active profile found for API key", log.String("api_key_name", apiKey.Name))
		return originalModel
	}

	// Find the active profile
	activeProfile, ok := lo.Find(profiles.Profiles, func(profile objects.APIKeyProfile) bool {
		return profile.Name == profiles.ActiveProfile
	})

	if !ok {
		log.Warn(ctx, "Active profile not found in profiles list",
			log.String("active_profile", profiles.ActiveProfile),
			log.String("api_key_name", apiKey.Name))

		return originalModel
	}

	// Apply model mapping
	mappedModel := m.applyModelMapping(activeProfile.ModelMappings, originalModel)

	if mappedModel != originalModel {
		log.Debug(ctx, "Model mapped using API key profile",
			log.String("api_key_name", apiKey.Name),
			log.String("active_profile", profiles.ActiveProfile),
			log.String("original_model", originalModel),
			log.String("mapped_model", mappedModel))
	} else {
		log.Debug(ctx, "Model not mapped using API key profile",
			log.String("api_key_name", apiKey.Name),
			log.String("active_profile", profiles.ActiveProfile),
			log.String("original_model", originalModel))
	}

	return mappedModel
}

// applyModelMapping applies model mappings from the given list
// Returns the mapped model or the original if no mapping is found.
func (m *ModelMapper) applyModelMapping(mappings []objects.ModelMapping, model string) string {
	for _, mapping := range mappings {
		if m.matchesMapping(mapping.From, model) {
			return mapping.To
		}
	}

	return model
}

// matchesMapping checks if a model matches a mapping pattern using cached regex
// Supports exact match and regex patterns (including wildcard conversion).
func (m *ModelMapper) matchesMapping(pattern, model string) bool {
	cached := m.getOrCreatePattern(pattern)

	// If regex compilation failed, fall back to exact match
	if cached.compileErr {
		return cached.exactMatch && pattern == model
	}

	// Use exact match for simple patterns
	if cached.exactMatch {
		return pattern == model
	}

	// Use compiled regex for pattern matching
	return cached.regex.MatchString(model)
}

// getOrCreatePattern retrieves or creates a cached pattern for the given input.
func (m *ModelMapper) getOrCreatePattern(pattern string) *patternCache {
	m.mu.RLock()

	if cached, exists := m.cache[pattern]; exists {
		m.mu.RUnlock()
		return cached
	}

	m.mu.RUnlock()

	// Create new pattern cache entry
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if cached, exists := m.cache[pattern]; exists {
		return cached
	}

	cached := &patternCache{}

	// Check if it's a simple exact match (no special regex chars)
	if !containsRegexChars(pattern) {
		cached.exactMatch = true
		m.cache[pattern] = cached

		return cached
	}

	// Convert wildcard pattern to regex and compile
	regexPattern := convertToRegex(pattern)

	compiled, err := regexp.Compile("^" + regexPattern + "$")
	if err != nil {
		// Compilation failed, mark as compile error and use exact match fallback
		cached.compileErr = true
		cached.exactMatch = true
	} else {
		cached.regex = compiled
	}

	m.cache[pattern] = cached

	return cached
}

// containsRegexChars checks if pattern contains regex special characters.
func containsRegexChars(pattern string) bool {
	return strings.ContainsAny(pattern, "*?+[]{}()^$.|\\")
}

// convertToRegex converts wildcard patterns to regex patterns.
func convertToRegex(pattern string) string {
	// Escape regex special characters except * and .
	// We need to handle * and . specially for wildcard matching
	result := ""

	for _, char := range pattern {
		switch char {
		case '*':
			result += ".*"
		case '.':
			// In wildcard patterns, . should match any single character
			result += "."
		case '?':
			result += "."
		default:
			// Escape other regex special characters
			if strings.ContainsRune("^$+[]{}()|\\", char) {
				result += "\\" + string(char)
			} else {
				result += string(char)
			}
		}
	}

	return result
}

// GetActiveProfile returns the active profile for an API key, if any.
func (m *ModelMapper) GetActiveProfile(apiKey *ent.APIKey) *objects.APIKeyProfile {
	if apiKey == nil || apiKey.Profiles == nil || apiKey.Profiles.ActiveProfile == "" {
		return nil
	}

	for i := range apiKey.Profiles.Profiles {
		if apiKey.Profiles.Profiles[i].Name == apiKey.Profiles.ActiveProfile {
			return &apiKey.Profiles.Profiles[i]
		}
	}

	return nil
}

// HasActiveProfile checks if an API key has an active profile with model mappings.
func (m *ModelMapper) HasActiveProfile(apiKey *ent.APIKey) bool {
	profile := m.GetActiveProfile(apiKey)
	return profile != nil && len(profile.ModelMappings) > 0
}

// ClearCache clears the regex pattern cache (useful for testing or memory management).
func (m *ModelMapper) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cache = make(map[string]*patternCache)
}

// CacheSize returns the current number of cached patterns.
func (m *ModelMapper) CacheSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.cache)
}
