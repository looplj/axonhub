package objects

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

const (
	// MaxPluginIDLength is the manifest id limit and the plugins.plugin_id column width.
	MaxPluginIDLength = 64
	// MaxPluginNameLength, MaxPluginVersionLength, and MaxPluginDescriptionLength
	// match the plugins table string columns. They are not indexed.
	MaxPluginNameLength        = 1024
	MaxPluginVersionLength     = 128
	MaxPluginDescriptionLength = 4096

	ConfigTypeString   = "string"
	ConfigTypeInteger  = "integer"
	ConfigTypeNumber   = "number"
	ConfigTypeBoolean  = "boolean"
	ConfigFormatSecret = "secret"
)

var (
	pluginIDPattern = regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9-]+)+$`)
	pageKeyPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

	manifestFieldNames = map[string]struct{}{
		"schemaVersion":    {},
		"id":               {},
		"name":             {},
		"version":          {},
		"description":      {},
		"axonhubVersion":   {},
		"pluginApiVersion": {},
		"permissions":      {},
		"events":           {},
		"hooks":            {},
		"configSchema":     {},
		"ui":               {},
		"providers":        {},
	}
	hookFieldNames = map[string]struct{}{
		"request":     {},
		"attempt":     {},
		"response":    {},
		"streamChunk": {},
	}
	uiFieldNames = map[string]struct{}{
		"pages":  {},
		"panels": {},
	}
	pageFieldNames = map[string]struct{}{
		"key":   {},
		"entry": {},
		"title": {},
	}
	configSchemaFieldNames = map[string]struct{}{
		"type":       {},
		"properties": {},
	}
	configPropertyFieldNames = map[string]struct{}{
		"type":        {},
		"enum":        {},
		"minimum":     {},
		"maximum":     {},
		"default":     {},
		"title":       {},
		"description": {},
		"format":      {},
	}
	configTypes = map[string]struct{}{
		ConfigTypeString:  {},
		ConfigTypeInteger: {},
		ConfigTypeNumber:  {},
		ConfigTypeBoolean: {},
	}
)

// PluginManifest is the parsed manifest.json document.
// Closed fields are recorded as presence flags so a later check can reject
// them with a dedicated error. Their structure is intentionally not defined.
type PluginManifest struct {
	SchemaVersion       int                 `json:"schemaVersion,omitempty"`
	SchemaVersionSet    bool                `json:"-"`
	ID                  string              `json:"id,omitempty"`
	Name                string              `json:"name,omitempty"`
	Version             string              `json:"version,omitempty"`
	Description         string              `json:"description,omitempty"`
	AxonhubVersion      string              `json:"axonhubVersion,omitempty"`
	PluginAPIVersion    int                 `json:"pluginApiVersion,omitempty"`
	PluginAPIVersionSet bool                `json:"-"`
	Permissions         []string            `json:"permissions,omitempty"`
	Events              []string            `json:"events,omitempty"`
	EventsPresent       bool                `json:"-"`
	Hooks               *PluginHooksSpec    `json:"hooks,omitempty"`
	ConfigSchema        *PluginConfigSchema `json:"configSchema,omitempty"`
	UI                  *PluginUI           `json:"ui,omitempty"`
	ProvidersPresent    bool                `json:"-"`
}

// PluginHooksSpec holds the opened request-hook booleans.
// response and streamChunk are presence-only until those hooks are scheduled.
type PluginHooksSpec struct {
	Request            bool `json:"request,omitempty"`
	Attempt            bool `json:"attempt,omitempty"`
	ResponsePresent    bool `json:"-"`
	StreamChunkPresent bool `json:"-"`
}

// PluginUI is the opened page list. panels is presence-only.
type PluginUI struct {
	Pages         []PluginUIPage `json:"pages,omitempty"`
	PanelsPresent bool           `json:"-"`
}

// PluginUIPage is one sidebar page declared by the package.
type PluginUIPage struct {
	Key   string      `json:"key"`
	Entry string      `json:"entry"`
	Title PluginTitle `json:"title"`
}

// PluginConfigSchema is the subset of JSON Schema accepted for plugin config.
// Property names are dynamic and are not a fixed-field whitelist.
type PluginConfigSchema struct {
	Type       string                          `json:"type"`
	Properties map[string]PluginConfigProperty `json:"properties,omitempty"`
}

// PluginConfigProperty is one configSchema property.
type PluginConfigProperty struct {
	Type        string   `json:"type"`
	Enum        []any    `json:"enum,omitempty"`
	Minimum     *float64 `json:"minimum,omitempty"`
	Maximum     *float64 `json:"maximum,omitempty"`
	Default     any      `json:"default,omitempty"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Format      string   `json:"format,omitempty"`
}

// IsSecret reports whether the property is marked format secret.
func (p PluginConfigProperty) IsSecret() bool {
	return p.Format == ConfigFormatSecret
}

type pluginTitleLang struct {
	Lang string
	Text string
}

// PluginTitle is either a plain string or a language map.
// Marshaling writes the same form that was decoded, including language-key order.
type PluginTitle struct {
	text   string
	langs  []pluginTitleLang
	object bool
}

// Validate checks the shape of opened manifest fields.
// Version compatibility and "not enabled yet" rejections live in internal/server/plugin.
func (m *PluginManifest) Validate() error {
	if m == nil {
		return errors.New("manifest is required")
	}
	if !m.SchemaVersionSet {
		return errors.New("schemaVersion is required")
	}
	if m.SchemaVersion != 1 {
		return errors.New("schemaVersion must be 1")
	}
	if m.ID == "" {
		return errors.New("id is required")
	}
	if len(m.ID) > MaxPluginIDLength || !pluginIDPattern.MatchString(m.ID) {
		return fmt.Errorf("id %q must match %s and be at most %d characters", m.ID, pluginIDPattern.String(), MaxPluginIDLength)
	}
	if m.Name == "" {
		return errors.New("name is required")
	}
	if len(m.Name) > MaxPluginNameLength {
		return fmt.Errorf("name must be at most %d characters", MaxPluginNameLength)
	}
	if m.Version == "" {
		return errors.New("version is required")
	}
	if len(m.Version) > MaxPluginVersionLength {
		return fmt.Errorf("version must be at most %d characters", MaxPluginVersionLength)
	}
	if _, err := semver.NewVersion(m.Version); err != nil {
		return fmt.Errorf("version %q is not valid semver: %w", m.Version, err)
	}
	if len(m.Description) > MaxPluginDescriptionLength {
		return fmt.Errorf("description must be at most %d characters", MaxPluginDescriptionLength)
	}
	if strings.TrimSpace(m.AxonhubVersion) == "" {
		return errors.New("axonhubVersion is required")
	}
	if !m.PluginAPIVersionSet {
		return errors.New("pluginApiVersion is required")
	}
	if m.ConfigSchema != nil {
		if err := m.ConfigSchema.Validate(); err != nil {
			return err
		}
	}
	if m.UI != nil {
		if err := m.UI.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks configSchema shape. Property names are not whitelisted.
func (s *PluginConfigSchema) Validate() error {
	if s == nil {
		return nil
	}
	if s.Type == "" {
		return errors.New("configSchema.type is required")
	}
	if s.Type != "object" {
		return errors.New("configSchema.type must be object")
	}
	for _, name := range slices.Sorted(maps.Keys(s.Properties)) {
		if name == "" {
			return errors.New("configSchema.properties contains an empty name")
		}
		if err := s.Properties[name].validate("configSchema.properties." + name); err != nil {
			return err
		}
	}
	return nil
}

func (p PluginConfigProperty) validate(fieldPath string) error {
	if _, ok := configTypes[p.Type]; !ok {
		return fmt.Errorf("%s.type must be string, integer, number, or boolean", fieldPath)
	}
	if p.Format != "" && p.Format != ConfigFormatSecret {
		return fmt.Errorf("%s.format must be secret", fieldPath)
	}
	if p.Format == ConfigFormatSecret && p.Type != ConfigTypeString {
		return fmt.Errorf("%s.format secret is only valid for string", fieldPath)
	}
	if p.Minimum != nil || p.Maximum != nil {
		if p.Type != ConfigTypeInteger && p.Type != ConfigTypeNumber {
			return fmt.Errorf("%s numeric bounds are only valid for numeric types", fieldPath)
		}
		if p.Minimum != nil && (math.IsNaN(*p.Minimum) || math.IsInf(*p.Minimum, 0)) {
			return fmt.Errorf("%s.minimum must be a number", fieldPath)
		}
		if p.Maximum != nil && (math.IsNaN(*p.Maximum) || math.IsInf(*p.Maximum, 0)) {
			return fmt.Errorf("%s.maximum must be a number", fieldPath)
		}
		if p.Minimum != nil && p.Maximum != nil && *p.Minimum > *p.Maximum {
			return fmt.Errorf("%s.minimum is greater than maximum", fieldPath)
		}
	}
	if p.Enum != nil && len(p.Enum) == 0 {
		return fmt.Errorf("%s.enum must not be empty", fieldPath)
	}
	for i, item := range p.Enum {
		if !valueMatchesType(p.Type, item) {
			return fmt.Errorf("%s.enum[%d] must be %s", fieldPath, i, p.Type)
		}
	}
	if p.Default != nil {
		if err := p.CheckValue(p.Default); err != nil {
			return fmt.Errorf("%s.default: %w", fieldPath, err)
		}
	}
	return nil
}

// CheckValue reports whether value satisfies this property.
// JSON numbers that are whole numbers satisfy integer. Strings are never coerced.
// The error does not include the value.
func (p PluginConfigProperty) CheckValue(value any) error {
	if value == nil || !valueMatchesType(p.Type, value) {
		return fmt.Errorf("expected %s", p.Type)
	}
	if len(p.Enum) > 0 && !enumContains(p.Enum, value) {
		return errors.New("value is not in enum")
	}
	if p.Minimum != nil || p.Maximum != nil {
		number, ok := numericValue(value)
		if !ok {
			return fmt.Errorf("expected %s", p.Type)
		}
		if p.Minimum != nil && number < *p.Minimum {
			return errors.New("value is below minimum")
		}
		if p.Maximum != nil && number > *p.Maximum {
			return errors.New("value is above maximum")
		}
	}
	return nil
}

// Validate checks page shape. Entry existence inside the ZIP is checked by the package parser.
func (u *PluginUI) Validate() error {
	if u == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(u.Pages))
	for i, page := range u.Pages {
		if err := page.Validate(); err != nil {
			return fmt.Errorf("ui.pages[%d]: %w", i, err)
		}
		if _, ok := seen[page.Key]; ok {
			return fmt.Errorf("ui.pages key %q is duplicated", page.Key)
		}
		seen[page.Key] = struct{}{}
	}
	return nil
}

// Validate checks one page's key, entry path, and title.
func (p PluginUIPage) Validate() error {
	if !pageKeyPattern.MatchString(p.Key) {
		return fmt.Errorf("key %q must match %s", p.Key, pageKeyPattern.String())
	}
	if err := validateUIEntry(p.Entry); err != nil {
		return err
	}
	if err := p.Title.Validate(); err != nil {
		return fmt.Errorf("title: %w", err)
	}
	return nil
}

func validateUIEntry(entry string) error {
	if entry == "" {
		return errors.New("entry is required")
	}
	if strings.Contains(entry, "\\") || strings.ContainsRune(entry, 0) {
		return fmt.Errorf("entry %q is not a valid ui html path", entry)
	}
	if strings.HasPrefix(entry, "/") || path.IsAbs(entry) {
		return fmt.Errorf("entry %q must not be absolute", entry)
	}
	parts := strings.Split(entry, "/")
	if len(parts) < 2 || parts[0] != "ui" {
		return fmt.Errorf("entry %q must be under ui/", entry)
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("entry %q must not contain %q", entry, part)
		}
	}
	file := parts[len(parts)-1]
	if !strings.HasSuffix(file, ".html") || file == ".html" {
		return fmt.Errorf("entry %q must be an html file under ui/", entry)
	}
	return nil
}

// Validate checks the title union: a non-blank string, or a language map that includes en.
func (t PluginTitle) Validate() error {
	if !t.object {
		if strings.TrimSpace(t.text) == "" {
			return errors.New("title must not be blank")
		}
		return nil
	}
	if len(t.langs) == 0 {
		return errors.New("title must include en")
	}
	hasEN := false
	seen := make(map[string]struct{}, len(t.langs))
	for _, lang := range t.langs {
		if strings.TrimSpace(lang.Lang) == "" {
			return errors.New("title language key must not be empty")
		}
		if _, ok := seen[lang.Lang]; ok {
			return fmt.Errorf("title language %q is duplicated", lang.Lang)
		}
		seen[lang.Lang] = struct{}{}
		if strings.TrimSpace(lang.Text) == "" {
			return fmt.Errorf("title language %q must be a non-empty string", lang.Lang)
		}
		if lang.Lang == "en" {
			hasEN = true
		}
	}
	if !hasEN {
		return errors.New("title must include en")
	}
	return nil
}

// IsZero reports whether the title was never set.
func (t PluginTitle) IsZero() bool {
	return !t.object && t.text == "" && len(t.langs) == 0
}

// UnmarshalJSON accepts a string or a language map and preserves that form.
func (t *PluginTitle) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return errors.New("title must not be null")
	}
	switch data[0] {
	case '"':
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return errors.New("title must be a string or a language map")
		}
		t.text = text
		t.object = false
		t.langs = nil
		return t.Validate()
	case '{':
		langs, err := decodeTitleLanguages(data)
		if err != nil {
			return err
		}
		t.object = true
		t.langs = langs
		t.text = ""
		return t.Validate()
	default:
		return errors.New("title must be a string or a language map")
	}
}

// MarshalJSON writes the original title form.
func (t PluginTitle) MarshalJSON() ([]byte, error) {
	if t.object {
		var buf bytes.Buffer
		buf.WriteByte('{')
		for i, lang := range t.langs {
			if i > 0 {
				buf.WriteByte(',')
			}
			key, err := json.Marshal(lang.Lang)
			if err != nil {
				return nil, err
			}
			value, err := json.Marshal(lang.Text)
			if err != nil {
				return nil, err
			}
			buf.Write(key)
			buf.WriteByte(':')
			buf.Write(value)
		}
		buf.WriteByte('}')
		return buf.Bytes(), nil
	}
	return json.Marshal(t.text)
}

func decodeTitleLanguages(data []byte) ([]pluginTitleLang, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	token, err := dec.Token()
	if err != nil {
		return nil, errors.New("title must be a string or a language map")
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return nil, errors.New("title must be a string or a language map")
	}
	langs := make([]pluginTitleLang, 0)
	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			return nil, errors.New("title must be a string or a language map")
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, errors.New("title must be a string or a language map")
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, errors.New("title must be a string or a language map")
		}
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || raw[0] != '"' {
			return nil, fmt.Errorf("title language %q must be a non-empty string", key)
		}
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, fmt.Errorf("title language %q must be a non-empty string", key)
		}
		langs = append(langs, pluginTitleLang{Lang: key, Text: text})
	}
	end, err := dec.Token()
	if err != nil {
		return nil, errors.New("title must be a string or a language map")
	}
	endDelim, ok := end.(json.Delim)
	if !ok || endDelim != '}' {
		return nil, errors.New("title must be a string or a language map")
	}
	return langs, nil
}

// UnmarshalJSON decodes a manifest with an explicit field whitelist.
func (m *PluginManifest) UnmarshalJSON(data []byte) error {
	obj, err := decodeObject(data, "")
	if err != nil {
		return err
	}
	if err := rejectUnknownFields(obj, manifestFieldNames, ""); err != nil {
		return err
	}
	if raw, ok := obj["schemaVersion"]; ok {
		value, err := decodeJSONInt(raw, "schemaVersion")
		if err != nil {
			return err
		}
		m.SchemaVersion = value
		m.SchemaVersionSet = true
	}
	if raw, ok := obj["id"]; ok {
		value, err := decodeJSONString(raw, "id")
		if err != nil {
			return err
		}
		m.ID = value
	}
	if raw, ok := obj["name"]; ok {
		value, err := decodeJSONString(raw, "name")
		if err != nil {
			return err
		}
		m.Name = value
	}
	if raw, ok := obj["version"]; ok {
		value, err := decodeJSONString(raw, "version")
		if err != nil {
			return err
		}
		m.Version = value
	}
	if raw, ok := obj["description"]; ok {
		value, err := decodeJSONString(raw, "description")
		if err != nil {
			return err
		}
		m.Description = value
	}
	if raw, ok := obj["axonhubVersion"]; ok {
		value, err := decodeJSONString(raw, "axonhubVersion")
		if err != nil {
			return err
		}
		m.AxonhubVersion = value
	}
	if raw, ok := obj["pluginApiVersion"]; ok {
		value, err := decodeJSONInt(raw, "pluginApiVersion")
		if err != nil {
			return err
		}
		m.PluginAPIVersion = value
		m.PluginAPIVersionSet = true
	}
	m.Permissions = []string{}
	if raw, ok := obj["permissions"]; ok {
		values, err := decodeJSONStringSlice(raw, "permissions")
		if err != nil {
			return err
		}
		m.Permissions = values
	}
	if raw, ok := obj["events"]; ok {
		m.EventsPresent = true
		values, err := decodeJSONStringSlice(raw, "events")
		if err != nil {
			return err
		}
		m.Events = values
	}
	if _, ok := obj["providers"]; ok {
		m.ProvidersPresent = true
	}
	if raw, ok := obj["hooks"]; ok {
		var hooks PluginHooksSpec
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return err
		}
		m.Hooks = &hooks
	}
	if raw, ok := obj["configSchema"]; ok {
		var schema PluginConfigSchema
		if err := json.Unmarshal(raw, &schema); err != nil {
			return err
		}
		m.ConfigSchema = &schema
	}
	if raw, ok := obj["ui"]; ok {
		var ui PluginUI
		if err := json.Unmarshal(raw, &ui); err != nil {
			return err
		}
		m.UI = &ui
	}
	return nil
}

// UnmarshalJSON decodes hooks and records unopened response fields without parsing them.
func (h *PluginHooksSpec) UnmarshalJSON(data []byte) error {
	raw, err := decodeObject(data, "hooks")
	if err != nil {
		return err
	}
	if err := rejectUnknownFields(raw, hookFieldNames, "hooks"); err != nil {
		return err
	}
	if _, ok := raw["response"]; ok {
		h.ResponsePresent = true
	}
	if _, ok := raw["streamChunk"]; ok {
		h.StreamChunkPresent = true
	}
	if value, ok := raw["request"]; ok {
		parsed, err := decodeJSONBool(value, "hooks.request")
		if err != nil {
			return err
		}
		h.Request = parsed
	}
	if value, ok := raw["attempt"]; ok {
		parsed, err := decodeJSONBool(value, "hooks.attempt")
		if err != nil {
			return err
		}
		h.Attempt = parsed
	}
	return nil
}

// UnmarshalJSON decodes pages and records ui.panels without parsing it.
func (u *PluginUI) UnmarshalJSON(data []byte) error {
	raw, err := decodeObject(data, "ui")
	if err != nil {
		return err
	}
	if err := rejectUnknownFields(raw, uiFieldNames, "ui"); err != nil {
		return err
	}
	if _, ok := raw["panels"]; ok {
		u.PanelsPresent = true
	}
	pagesRaw, ok := raw["pages"]
	if !ok {
		return nil
	}
	pagesRaw = bytes.TrimSpace(pagesRaw)
	if len(pagesRaw) == 0 || pagesRaw[0] != '[' {
		return errors.New("ui.pages must be an array")
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(pagesRaw, &elems); err != nil {
		return errors.New("ui.pages must be an array")
	}
	pages := make([]PluginUIPage, 0, len(elems))
	for i, elem := range elems {
		var page PluginUIPage
		if err := page.unmarshalJSONAt(elem, fmt.Sprintf("ui.pages[%d]", i)); err != nil {
			return err
		}
		pages = append(pages, page)
	}
	u.Pages = pages
	return nil
}

// UnmarshalJSON decodes one page with a fixed field whitelist.
func (p *PluginUIPage) UnmarshalJSON(data []byte) error {
	return p.unmarshalJSONAt(data, "ui.pages")
}

func (p *PluginUIPage) unmarshalJSONAt(data []byte, fieldPath string) error {
	raw, err := decodeObject(data, fieldPath)
	if err != nil {
		return err
	}
	if err := rejectUnknownFields(raw, pageFieldNames, fieldPath); err != nil {
		return err
	}
	keyRaw, ok := raw["key"]
	if !ok {
		return fmt.Errorf("%s.key is required", fieldPath)
	}
	key, err := decodeJSONString(keyRaw, fieldPath+".key")
	if err != nil {
		return err
	}
	entryRaw, ok := raw["entry"]
	if !ok {
		return fmt.Errorf("%s.entry is required", fieldPath)
	}
	entry, err := decodeJSONString(entryRaw, fieldPath+".entry")
	if err != nil {
		return err
	}
	titleRaw, ok := raw["title"]
	if !ok {
		return fmt.Errorf("%s.title is required", fieldPath)
	}
	var title PluginTitle
	if err := json.Unmarshal(titleRaw, &title); err != nil {
		return fmt.Errorf("%s.title: %w", fieldPath, err)
	}
	p.Key = key
	p.Entry = entry
	p.Title = title
	return nil
}

// UnmarshalJSON decodes configSchema with a fixed keyword whitelist.
// Names under properties are dynamic.
func (s *PluginConfigSchema) UnmarshalJSON(data []byte) error {
	raw, err := decodeObject(data, "configSchema")
	if err != nil {
		return err
	}
	if err := rejectUnknownFields(raw, configSchemaFieldNames, "configSchema"); err != nil {
		return err
	}
	if value, ok := raw["type"]; ok {
		parsed, err := decodeJSONString(value, "configSchema.type")
		if err != nil {
			return err
		}
		s.Type = parsed
	}
	propsRaw, ok := raw["properties"]
	if !ok {
		return nil
	}
	props, err := decodeObject(propsRaw, "configSchema.properties")
	if err != nil {
		return err
	}
	s.Properties = make(map[string]PluginConfigProperty, len(props))
	for name, propRaw := range props {
		var prop PluginConfigProperty
		if err := prop.unmarshalJSONAt(propRaw, "configSchema.properties."+name); err != nil {
			return err
		}
		s.Properties[name] = prop
	}
	return nil
}

func (p *PluginConfigProperty) unmarshalJSONAt(data []byte, fieldPath string) error {
	raw, err := decodeObject(data, fieldPath)
	if err != nil {
		return err
	}
	if err := rejectUnknownFields(raw, configPropertyFieldNames, fieldPath); err != nil {
		return err
	}
	if value, ok := raw["type"]; ok {
		parsed, err := decodeJSONString(value, fieldPath+".type")
		if err != nil {
			return err
		}
		p.Type = parsed
	}
	if value, ok := raw["enum"]; ok {
		parsed, err := decodeJSONArray(value, fieldPath+".enum")
		if err != nil {
			return err
		}
		p.Enum = parsed
	}
	if value, ok := raw["minimum"]; ok {
		parsed, err := decodeJSONFloat(value, fieldPath+".minimum")
		if err != nil {
			return err
		}
		p.Minimum = &parsed
	}
	if value, ok := raw["maximum"]; ok {
		parsed, err := decodeJSONFloat(value, fieldPath+".maximum")
		if err != nil {
			return err
		}
		p.Maximum = &parsed
	}
	if value, ok := raw["default"]; ok {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("%s.default must not be null", fieldPath)
		}
		parsed, err := decodeJSONValue(value)
		if err != nil {
			return fmt.Errorf("%s.default must be a JSON value", fieldPath)
		}
		p.Default = parsed
	}
	if value, ok := raw["title"]; ok {
		parsed, err := decodeJSONString(value, fieldPath+".title")
		if err != nil {
			return err
		}
		p.Title = parsed
	}
	if value, ok := raw["description"]; ok {
		parsed, err := decodeJSONString(value, fieldPath+".description")
		if err != nil {
			return err
		}
		p.Description = parsed
	}
	if value, ok := raw["format"]; ok {
		parsed, err := decodeJSONString(value, fieldPath+".format")
		if err != nil {
			return err
		}
		p.Format = parsed
	}
	return nil
}

func decodeObject(data []byte, fieldPath string) (map[string]json.RawMessage, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) || data[0] != '{' {
		return nil, objectRequired(fieldPath)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, objectRequired(fieldPath)
	}
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	return raw, nil
}

func objectRequired(fieldPath string) error {
	if fieldPath == "" {
		return errors.New("manifest must be an object")
	}
	return fmt.Errorf("%s must be an object", fieldPath)
}

func rejectUnknownFields(raw map[string]json.RawMessage, allowed map[string]struct{}, fieldPath string) error {
	unknown := make([]string, 0)
	for key := range raw {
		if _, ok := allowed[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	slices.Sort(unknown)
	full := unknown[0]
	if fieldPath != "" {
		full = fieldPath + "." + full
	}
	return fmt.Errorf("unknown field %s", full)
}

func decodeJSONString(data []byte, fieldPath string) (string, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '"' {
		return "", fmt.Errorf("%s must be a string", fieldPath)
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", fieldPath)
	}
	return value, nil
}

func decodeJSONBool(data []byte, fieldPath string) (bool, error) {
	data = bytes.TrimSpace(data)
	switch string(data) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be a boolean", fieldPath)
	}
}

func decodeJSONInt(data []byte, fieldPath string) (int, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] == '"' || data[0] == 't' || data[0] == 'f' || data[0] == 'n' || data[0] == '{' || data[0] == '[' {
		return 0, fmt.Errorf("%s must be an integer", fieldPath)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var num json.Number
	if err := dec.Decode(&num); err != nil {
		return 0, fmt.Errorf("%s must be an integer", fieldPath)
	}
	if strings.ContainsAny(num.String(), ".eE") {
		return 0, fmt.Errorf("%s must be an integer", fieldPath)
	}
	parsed, err := strconv.Atoi(num.String())
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", fieldPath)
	}
	return parsed, nil
}

func decodeJSONFloat(data []byte, fieldPath string) (float64, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] == '"' || data[0] == 't' || data[0] == 'f' || data[0] == 'n' || data[0] == '{' || data[0] == '[' {
		return 0, fmt.Errorf("%s must be a number", fieldPath)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var num json.Number
	if err := dec.Decode(&num); err != nil {
		return 0, fmt.Errorf("%s must be a number", fieldPath)
	}
	parsed, err := num.Float64()
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, fmt.Errorf("%s must be a number", fieldPath)
	}
	return parsed, nil
}

func decodeJSONStringSlice(data []byte, fieldPath string) ([]string, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '[' {
		return nil, fmt.Errorf("%s must be an array of strings", fieldPath)
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, fmt.Errorf("%s must be an array of strings", fieldPath)
	}
	values := make([]string, 0, len(raws))
	for i, raw := range raws {
		value, err := decodeJSONString(raw, fmt.Sprintf("%s[%d]", fieldPath, i))
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func decodeJSONArray(data []byte, fieldPath string) ([]any, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '[' {
		return nil, fmt.Errorf("%s must be an array", fieldPath)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var values []any
	if err := dec.Decode(&values); err != nil {
		return nil, fmt.Errorf("%s must be an array", fieldPath)
	}
	if values == nil {
		values = []any{}
	}
	return values, nil
}

func decodeJSONValue(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func valueMatchesType(typ string, value any) bool {
	switch typ {
	case ConfigTypeString:
		_, ok := value.(string)
		return ok
	case ConfigTypeBoolean:
		_, ok := value.(bool)
		return ok
	case ConfigTypeNumber:
		_, ok := numericValue(value)
		return ok
	case ConfigTypeInteger:
		number, ok := numericValue(value)
		return ok && isWhole(number)
	default:
		return false
	}
}

func enumContains(enum []any, value any) bool {
	for _, item := range enum {
		if sameConfigValue(item, value) {
			return true
		}
	}
	return false
}

func sameConfigValue(left, right any) bool {
	if text, ok := left.(string); ok {
		other, ok := right.(string)
		return ok && text == other
	}
	if flag, ok := left.(bool); ok {
		other, ok := right.(bool)
		return ok && flag == other
	}
	leftNumber, leftOK := numericValue(left)
	rightNumber, rightOK := numericValue(right)
	return leftOK && rightOK && leftNumber == rightNumber
}

func numericValue(value any) (float64, bool) {
	switch number := value.(type) {
	case json.Number:
		parsed, err := number.Float64()
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return 0, false
		}
		return parsed, true
	case float64:
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return 0, false
		}
		return number, true
	case float32:
		parsed := float64(number)
		if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return 0, false
		}
		return parsed, true
	case int:
		return float64(number), true
	case int8:
		return float64(number), true
	case int16:
		return float64(number), true
	case int32:
		return float64(number), true
	case int64:
		return float64(number), true
	case uint:
		return float64(number), true
	case uint8:
		return float64(number), true
	case uint16:
		return float64(number), true
	case uint32:
		return float64(number), true
	case uint64:
		return float64(number), true
	default:
		return 0, false
	}
}

func isWhole(number float64) bool {
	return !math.IsNaN(number) && !math.IsInf(number, 0) && number == math.Trunc(number)
}
