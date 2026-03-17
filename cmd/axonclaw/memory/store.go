package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
)

type HookCaller interface {
	CallHook(ctx context.Context, hook string, input any) ([]json.RawMessage, error)
}

type StoreOptions struct {
	Layout   Layout
	Embedder Embedder
	Hooks    HookCaller
	Now      func() time.Time
}

type Store struct {
	layout   Layout
	embedder Embedder
	hooks    HookCaller
	now      func() time.Time
	db       dbState
}

type FileEntry struct {
	Label string
	Size  int64
}

type SearchMatch struct {
	Label  string  `json:"label"`
	Line   int     `json:"line"`
	Text   string  `json:"text"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason,omitempty"`
	Source string  `json:"source,omitempty"`
}

type MemoryAddHookInput struct {
	Workspace string   `json:"workspace"`
	Content   string   `json:"content"`
	LongTerm  bool     `json:"long_term"`
	Targets   []string `json:"targets"`
	Timestamp string   `json:"timestamp"`
}

type MemoryAddHookOutput struct {
	ExtraEntries []PluginMemoryEntry `json:"extra_entries"`
}

type PluginMemoryEntry struct {
	Target  string `json:"target"`
	Content string `json:"content"`
}

type MemorySearchHookInput struct {
	Workspace string        `json:"workspace"`
	Query     string        `json:"query"`
	Limit     int           `json:"limit"`
	Results   []SearchMatch `json:"results"`
}

type MemorySearchHookOutput struct {
	ReplaceResults bool          `json:"replace_results,omitempty"`
	Results        []SearchMatch `json:"results"`
}

func NewStore(opts StoreOptions) *Store {
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	return &Store{
		layout:   opts.Layout,
		embedder: opts.Embedder,
		hooks:    opts.Hooks,
		now:      now,
	}
}

func (s *Store) Add(ctx context.Context, content string, longTerm bool) ([]string, bool, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, false, fmt.Errorf("content is required")
	}

	now := s.now()
	targets := []string{s.layout.DailyPath(now)}
	autoPromoted := false
	if longTerm || shouldPromoteToLongTerm(content) {
		targets = append(targets, s.layout.LongTermPath())
		autoPromoted = !longTerm
	}

	for _, target := range targets {
		if err := appendMemoryFile(target, formatMemoryEntry(content, now)); err != nil {
			return nil, false, err
		}
	}

	if err := s.ensureIndexed(ctx); err != nil {
		return nil, false, err
	}

	if err := s.applyAddHooks(ctx, content, longTerm, targets, now); err != nil {
		return nil, false, err
	}

	if err := s.ensureIndexed(ctx); err != nil {
		return nil, false, err
	}

	return targets, autoPromoted, nil
}

func (s *Store) Get(date string, longTerm, yesterday bool) (string, error) {
	path, err := s.resolveMemoryTarget(date, longTerm, yesterday)
	if err != nil {
		return "", err
	}

	return readMemoryFile(path)
}

func (s *Store) List() ([]FileEntry, error) {
	var out []FileEntry

	if info, err := os.Stat(s.layout.LongTermPath()); err == nil {
		out = append(out, FileEntry{Label: ".axonclaw/MEMORY.md", Size: info.Size()})
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat long-term memory: %w", err)
	}

	entries, err := os.ReadDir(s.layout.DailyDir())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read memory directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat memory file %q: %w", entry.Name(), err)
		}

		out = append(out, FileEntry{
			Label: filepathToSlash(".axonclaw/memory/" + entry.Name()),
			Size:  info.Size(),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Label < out[j].Label
	})

	return out, nil
}

func (s *Store) Search(ctx context.Context, query string, limit int) ([]SearchMatch, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	if limit <= 0 {
		limit = 10
	}

	if err := s.ensureIndexed(ctx); err != nil {
		return nil, err
	}

	chunks, err := s.loadIndexedChunks(ctx)
	if err != nil {
		return nil, err
	}

	var queryEmbedding []float64
	if s.embedder != nil {
		vectors, err := s.embedder.Embed(ctx, []string{query})
		if err == nil && len(vectors) == 1 {
			queryEmbedding = vectors[0]
		}
	}

	matches := make([]SearchMatch, 0, len(chunks))
	for _, chunk := range chunks {
		keywordScore := lexicalScore(query, chunk.Heading+"\n"+chunk.Content)
		semanticScore := cosineSimilarity(queryEmbedding, chunk.Embedding)
		if keywordScore == 0 && semanticScore == 0 {
			continue
		}

		reason := "keyword"
		score := keywordScore
		if semanticScore > 0 {
			score = 0.45*keywordScore + 0.55*semanticScore
			reason = "hybrid"
			if keywordScore == 0 {
				reason = "semantic"
				score = semanticScore
			}
		}

		matches = append(matches, SearchMatch{
			Label:  chunk.Label,
			Line:   chunk.LineStart,
			Text:   chunk.Content,
			Score:  score,
			Reason: reason,
			Source: chunk.Scope,
		})
	}

	sortSearchMatches(matches)
	if len(matches) > limit {
		matches = matches[:limit]
	}

	hookOutputs, err := callHook[MemorySearchHookOutput](ctx, s.hooks, "memory.search", MemorySearchHookInput{
		Workspace: s.layout.Workspace(),
		Query:     query,
		Limit:     limit,
		Results:   matches,
	})
	if err != nil {
		return nil, err
	}

	for _, hookOutput := range hookOutputs {
		if hookOutput.ReplaceResults {
			matches = append([]SearchMatch(nil), hookOutput.Results...)
			continue
		}
		matches = append(matches, hookOutput.Results...)
	}

	matches = dedupeSearchMatches(matches)
	sortSearchMatches(matches)
	if len(matches) > limit {
		matches = matches[:limit]
	}

	return matches, nil
}

func (s *Store) RewriteLongTerm(ctx context.Context, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("content is required")
	}

	oldContent, err := readMemoryFile(s.layout.LongTermPath())
	if err != nil {
		return err
	}

	if oldContent != "" {
		now := s.now()
		archiveHeader := fmt.Sprintf("\n---\n\n## Archived from MEMORY.md (%s)\n\n", now.Format("2006-01-02 15:04"))
		if err := appendMemoryFile(s.layout.DailyPath(now), archiveHeader+oldContent+"\n"); err != nil {
			return fmt.Errorf("archive old long-term memory: %w", err)
		}
	}

	if err := writeMemoryFile(s.layout.LongTermPath(), content+"\n"); err != nil {
		return err
	}

	return s.ensureIndexed(ctx)
}

func (s *Store) Delete(ctx context.Context, date string, longTerm bool) (string, bool, error) {
	path, err := s.resolveDeleteTarget(date, longTerm)
	if err != nil {
		return "", false, err
	}

	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if indexErr := s.ensureIndexed(ctx); indexErr != nil {
				return "", false, indexErr
			}
			return path, false, nil
		}
		return "", false, err
	}

	if err := s.ensureIndexed(ctx); err != nil {
		return "", false, err
	}

	return path, true, nil
}

func (s *Store) resolveMemoryTarget(date string, longTerm, yesterday bool) (string, error) {
	if longTerm {
		if date != "" || yesterday {
			return "", fmt.Errorf("--longterm cannot be combined with daily selectors")
		}
		return s.layout.LongTermPath(), nil
	}

	switch {
	case date != "" && yesterday:
		return "", fmt.Errorf("--date and --yesterday cannot be used together")
	case date != "":
		parsed, err := time.Parse("2006-01-02", date)
		if err != nil {
			return "", fmt.Errorf("invalid --date %q, expected YYYY-MM-DD", date)
		}
		return s.layout.DailyPath(parsed), nil
	case yesterday:
		return s.layout.DailyPath(s.now().AddDate(0, 0, -1)), nil
	default:
		return s.layout.DailyPath(s.now()), nil
	}
}

func (s *Store) resolveDeleteTarget(date string, longTerm bool) (string, error) {
	if longTerm {
		if date != "" {
			return "", fmt.Errorf("--longterm cannot be combined with --date")
		}
		return s.layout.LongTermPath(), nil
	}

	if date == "" {
		return "", fmt.Errorf("delete requires --date or --longterm")
	}

	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", fmt.Errorf("invalid --date %q, expected YYYY-MM-DD", date)
	}

	return s.layout.DailyPath(parsed), nil
}

func (s *Store) applyAddHooks(ctx context.Context, content string, longTerm bool, targets []string, now time.Time) error {
	hookOutputs, err := callHook[MemoryAddHookOutput](ctx, s.hooks, "memory.add", MemoryAddHookInput{
		Workspace: s.layout.Workspace(),
		Content:   content,
		LongTerm:  longTerm,
		Targets:   targets,
		Timestamp: now.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}

	for _, output := range hookOutputs {
		for _, entry := range output.ExtraEntries {
			target := strings.ToLower(strings.TrimSpace(entry.Target))
			switch target {
			case "", "daily":
				target = s.layout.DailyPath(now)
			case "longterm", "memory", "long-term":
				target = s.layout.LongTermPath()
			default:
				return fmt.Errorf("memory.add plugin returned unsupported target %q", entry.Target)
			}

			if strings.TrimSpace(entry.Content) == "" {
				continue
			}

			if err := appendMemoryFile(target, formatMemoryEntry(entry.Content, now)); err != nil {
				return err
			}
		}
	}

	return nil
}

func callHook[T any](ctx context.Context, hooks HookCaller, hook string, input any) ([]T, error) {
	if hooks == nil {
		return nil, nil
	}

	rawList, err := hooks.CallHook(ctx, hook, input)
	if err != nil {
		return nil, err
	}

	out := make([]T, 0, len(rawList))
	for _, raw := range rawList {
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}

		var decoded T
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil, fmt.Errorf("decode %s hook response: %w", hook, err)
		}
		out = append(out, decoded)
	}

	return out, nil
}

func appendMemoryFile(path, content string) error {
	if err := os.MkdirAll(filepathDir(path), 0o755); err != nil {
		return fmt.Errorf("create memory directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open memory file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("write memory: %w", err)
	}

	return nil
}

func writeMemoryFile(path, content string) error {
	if err := os.MkdirAll(filepathDir(path), 0o755); err != nil {
		return fmt.Errorf("create memory directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write memory file: %w", err)
	}

	return nil
}

func readMemoryFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read memory file: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}

func formatMemoryEntry(content string, now time.Time) string {
	return fmt.Sprintf("- [%s] %s\n", now.Format("15:04"), strings.TrimSpace(content))
}

func shouldPromoteToLongTerm(content string) bool {
	normalized := strings.ToLower(strings.TrimSpace(content))
	if normalized == "" {
		return false
	}

	keywords := []string{
		"user prefers", "prefer", "preference", "always", "never", "decision", "decided",
		"rule", "policy", "remember", "long-term", "durable", "stable", "lesson learned",
		"偏好", "习惯", "规则", "决策", "长期", "记住", "以后都",
	}
	for _, keyword := range keywords {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}

	return strings.HasPrefix(normalized, "remember:")
}

func lexicalScore(query, content string) float64 {
	query = normalizeText(query)
	content = normalizeText(content)
	if query == "" || content == "" {
		return 0
	}

	score := 0.0
	if strings.Contains(content, query) {
		score += 0.8
	}

	for _, token := range tokenize(query) {
		if token == "" {
			continue
		}
		count := strings.Count(content, token)
		if count == 0 {
			continue
		}
		score += math.Min(float64(count)*0.18, 0.5)
	}

	return math.Min(score, 1)
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}

	var dot float64
	var normA float64
	var normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	score := dot / (math.Sqrt(normA) * math.Sqrt(normB))
	if score < 0 {
		return 0
	}

	return score
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r), unicode.IsSpace(r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func tokenize(value string) []string {
	return strings.Fields(normalizeText(value))
}

func dedupeSearchMatches(matches []SearchMatch) []SearchMatch {
	seen := map[string]SearchMatch{}
	for _, match := range matches {
		key := fmt.Sprintf("%s:%d:%s", match.Label, match.Line, match.Text)
		if existing, ok := seen[key]; ok && existing.Score >= match.Score {
			continue
		}
		seen[key] = match
	}

	out := make([]SearchMatch, 0, len(seen))
	for _, match := range seen {
		out = append(out, match)
	}

	return out
}

func sortSearchMatches(matches []SearchMatch) {
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			if matches[i].Label == matches[j].Label {
				return matches[i].Line < matches[j].Line
			}
			return matches[i].Label < matches[j].Label
		}
		return matches[i].Score > matches[j].Score
	})
}

func filepathDir(path string) string {
	idx := strings.LastIndex(path, string(os.PathSeparator))
	if idx == -1 {
		return "."
	}
	return path[:idx]
}

func filepathToSlash(path string) string {
	return strings.ReplaceAll(path, string(os.PathSeparator), "/")
}
