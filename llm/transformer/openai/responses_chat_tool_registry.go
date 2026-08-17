package openai

import (
	"fmt"
	"maps"
)

type responsesChatToolNameAllocator struct {
	used map[string]toolIdentity
}

func newResponsesChatToolNameAllocator() *responsesChatToolNameAllocator {
	return &responsesChatToolNameAllocator{used: make(map[string]toolIdentity)}
}

// reserve returns a collision-free Chat function name.
func (allocator *responsesChatToolNameAllocator) reserve(preferred, fallback string, identity toolIdentity) (string, error) {
	if preferred == "" {
		preferred = fallback
	}
	if err := validateChatFunctionName(preferred); err == nil {
		if _, exists := allocator.used[preferred]; !exists {
			allocator.used[preferred] = identity
			return preferred, nil
		}
	}
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s_%d", fallback, i)
		if err := validateChatFunctionName(candidate); err != nil {
			return "", err
		}
		if _, exists := allocator.used[candidate]; !exists {
			allocator.used[candidate] = identity
			return candidate, nil
		}
	}
}

// reserveExact validates and assigns an identity-fixed Chat name.
func (allocator *responsesChatToolNameAllocator) reserveExact(name string, identity toolIdentity) error {
	if err := validateChatFunctionName(name); err != nil {
		return err
	}
	if owner, exists := allocator.used[name]; exists {
		if owner == identity {
			return nil
		}
		return toolNameCollisionError(name, owner, identity)
	}
	allocator.used[name] = identity
	return nil
}

type responsesChatToolMappingRegistry struct {
	byChatName            map[string]responsesChatToolMapping
	byIdentity            map[toolIdentity]responsesChatToolMapping
	identitiesByFlattened map[flattenedIdentityKey][]toolIdentity
	identitiesByNamespace map[string][]toolIdentity
	identitiesBySource    map[sourceIdentityKey][]toolIdentity
	identitiesByClient    map[string][]toolIdentity
	signatures            map[toolIdentity]string
}

func newResponsesChatToolMappingRegistry() *responsesChatToolMappingRegistry {
	return &responsesChatToolMappingRegistry{
		byChatName:            make(map[string]responsesChatToolMapping),
		byIdentity:            make(map[toolIdentity]responsesChatToolMapping),
		identitiesByFlattened: make(map[flattenedIdentityKey][]toolIdentity),
		identitiesByNamespace: make(map[string][]toolIdentity),
		identitiesBySource:    make(map[sourceIdentityKey][]toolIdentity),
		identitiesByClient:    make(map[string][]toolIdentity),
		signatures:            make(map[toolIdentity]string),
	}
}

func (registry *responsesChatToolMappingRegistry) find(identity toolIdentity) (responsesChatToolMapping, bool) {
	mapping, ok := registry.byIdentity[identity]
	return mapping, ok
}

func (registry *responsesChatToolMappingRegistry) signature(identity toolIdentity) (string, bool) {
	signature, ok := registry.signatures[identity]
	return signature, ok
}

func (registry *responsesChatToolMappingRegistry) saveSignature(identity toolIdentity, signature string) {
	registry.signatures[identity] = signature
}

func (registry *responsesChatToolMappingRegistry) add(
	identity toolIdentity,
	chatName string,
	signature string,
	mapping responsesChatToolMapping,
	historyOnly bool,
) {
	mapping.ChatName = chatName
	mapping.HistoryOnly = historyOnly
	registry.byIdentity[identity] = mapping
	registry.byChatName[chatName] = mapping
	registry.index(identity, mapping)
	if signature != "" {
		registry.signatures[identity] = signature
	}
}

func (registry *responsesChatToolMappingRegistry) index(identity toolIdentity, mapping responsesChatToolMapping) {
	flattened := namespaceChatName(identity.Name, identity.Namespace)
	flattenedKey := flattenedIdentityKey{Kind: identity.Kind, Name: flattened}
	registry.identitiesByFlattened[flattenedKey] = append(registry.identitiesByFlattened[flattenedKey], identity)
	if mapping.Namespace != "" {
		registry.identitiesByNamespace[mapping.Namespace] = append(
			registry.identitiesByNamespace[mapping.Namespace], identity,
		)
	}
	if mapping.SourceType != "" {
		sourceKey := sourceIdentityKey{SourceType: mapping.SourceType, Name: mapping.Name}
		registry.identitiesBySource[sourceKey] = append(registry.identitiesBySource[sourceKey], identity)
	}
	if identity.Kind == responsesChatToolClient {
		registry.identitiesByClient[identity.Name] = append(registry.identitiesByClient[identity.Name], identity)
	}
}

func (registry *responsesChatToolMappingRegistry) flattened(kind responsesChatToolKind, name string) []toolIdentity {
	return registry.identitiesByFlattened[flattenedIdentityKey{Kind: kind, Name: name}]
}

func (registry *responsesChatToolMappingRegistry) namespace(namespace string) []toolIdentity {
	return registry.identitiesByNamespace[namespace]
}

func (registry *responsesChatToolMappingRegistry) source(key sourceIdentityKey) []toolIdentity {
	return registry.identitiesBySource[key]
}

func (registry *responsesChatToolMappingRegistry) client(name string) []toolIdentity {
	return registry.identitiesByClient[name]
}

func (registry *responsesChatToolMappingRegistry) markActive(activeNames map[string]struct{}) {
	for identity, mapping := range registry.byIdentity {
		_, active := activeNames[mapping.ChatName]
		mapping.HistoryOnly = !active
		registry.byIdentity[identity] = mapping
		registry.byChatName[mapping.ChatName] = mapping
	}
}

func (registry *responsesChatToolMappingRegistry) mappings() map[string]responsesChatToolMapping {
	if len(registry.byChatName) == 0 {
		return nil
	}
	result := make(map[string]responsesChatToolMapping, len(registry.byChatName))
	maps.Copy(result, registry.byChatName)
	return result
}
