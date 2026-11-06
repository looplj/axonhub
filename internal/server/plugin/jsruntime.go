package plugin

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/syntax"
	"go.uber.org/zap"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

const (
	pluginEntry = "plugin.js"

	// pluginTopLevelTimeout bounds top-level execution during install checks.
	pluginTopLevelTimeout = time.Second
)

var errPluginTopLevelTimeout = errors.New("plugin.js top-level execution exceeded 1s")

// ExportFunc is one named export after top-level execution.
// A missing map key means the name was not exported.
// The default export is the key "default" and does not satisfy onEvent,
// onRequest, or onAttempt.
type ExportFunc struct {
	Function bool
	Async    bool
}

// ValidateExports checks the named exports a manifest's open capabilities require.
// It does not consult the field-open table, so tests can call it while events
// and request hooks are still closed.
// A nil exports map means the package has no root plugin.js.
func ValidateExports(manifest *objects.PluginManifest, exports map[string]ExportFunc) error {
	if manifest == nil {
		return errors.New("manifest is required")
	}
	required := requiredExports(manifest)
	if len(required) == 0 {
		return nil
	}
	if exports == nil {
		return errors.New("plugin.js is required")
	}
	for _, name := range required {
		exported, ok := exports[name]
		if !ok {
			return fmt.Errorf("plugin.js must export %s", name)
		}
		if !exported.Function {
			return fmt.Errorf("plugin.js export %s must be a function", name)
		}
	}
	return nil
}

func requiredExports(manifest *objects.PluginManifest) []string {
	required := make([]string, 0, 3)
	if len(manifest.Events) > 0 {
		required = append(required, "onEvent")
	}
	if manifest.Hooks != nil && manifest.Hooks.Request {
		required = append(required, "onRequest")
	}
	if manifest.Hooks != nil && manifest.Hooks.Attempt {
		required = append(required, "onAttempt")
	}
	return required
}

// pluginProgram is a linked in-package module graph.
// The linked module is what a runtime loads. compiled holds each module once
// so Link and import() return the same pointer for the same file.
type pluginProgram struct {
	module   *moejs.Module
	compiled map[string]*moejs.Module
}

func (p *pluginProgram) closure() map[string]struct{} {
	if p == nil {
		return nil
	}
	out := make(map[string]struct{}, len(p.compiled))
	for name := range p.compiled {
		out[name] = struct{}{}
	}
	return out
}

// compilePluginSources compiles and links the static import closure of plugin.js.
// It does not execute the module.
func compilePluginSources(contents *packageContents) (*pluginProgram, error) {
	if contents == nil || !contents.hasPluginJS() {
		return nil, errors.New("plugin.js is required")
	}
	prog := &pluginProgram{compiled: map[string]*moejs.Module{}}
	var compileOne func(name string) error
	compileOne = func(name string) error {
		if _, ok := prog.compiled[name]; ok {
			return nil
		}
		source, ok, err := contents.lookupSource(name)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("import %q does not exist in the package", name)
		}
		mod, err := moejs.Compile(name, string(source))
		if err != nil {
			return err
		}
		if err := rejectTopLevelAwait(name, string(source)); err != nil {
			return err
		}
		prog.compiled[name] = mod
		for _, specifier := range mod.Requests() {
			resolved, err := resolveImport(name, specifier)
			if err != nil {
				return err
			}
			if _, ok, err := contents.lookupSource(resolved); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("import %q does not exist in the package", specifier)
			}
			if err := compileOne(resolved); err != nil {
				return err
			}
		}
		return nil
	}
	if err := compileOne(pluginEntry); err != nil {
		return nil, err
	}
	linked, err := moejs.Link(prog.compiled[pluginEntry], prog.resolve)
	if err != nil {
		return nil, err
	}
	prog.module = linked
	return prog, nil
}

func (p *pluginProgram) resolve(referrer moejs.Referrer, specifier string) (*moejs.Module, error) {
	from := pluginEntry
	if referrer != nil && referrer.Name() != "" {
		from = referrer.Name()
	}
	resolved, err := resolveImport(from, specifier)
	if err != nil {
		return nil, err
	}
	mod, ok := p.compiled[resolved]
	if !ok {
		return nil, fmt.Errorf("import %q does not exist in the package", specifier)
	}
	return mod, nil
}

// rejectTopLevelAwait refuses await outside a function.
// Async hook exports are allowed; a top-level await is not.
func rejectTopLevelAwait(name, source string) error {
	parsed, err := syntax.ParseModule(name, source, syntax.Options{})
	if err != nil {
		return err
	}
	if !parsed.Async {
		return nil
	}
	return fmt.Errorf("%s: top-level await is not allowed", name)
}

// resolveImport accepts only package-relative specifiers that stay inside the archive.
func resolveImport(from, specifier string) (string, error) {
	if specifier == "" || strings.Contains(specifier, "\\") || strings.ContainsRune(specifier, 0) {
		return "", fmt.Errorf("import %q is not a package-relative path", specifier)
	}
	if strings.HasPrefix(specifier, "node:") {
		return "", fmt.Errorf("import %q is not a package-relative path", specifier)
	}
	if strings.HasPrefix(specifier, "/") || path.IsAbs(specifier) || windowsDrivePattern.MatchString(specifier) {
		return "", fmt.Errorf("import %q is absolute", specifier)
	}
	if !strings.HasPrefix(specifier, "./") && !strings.HasPrefix(specifier, "../") {
		return "", fmt.Errorf("import %q is not a package-relative path", specifier)
	}
	resolved := path.Clean(path.Join(path.Dir(from), specifier))
	if resolved == "." || resolved == ".." || strings.HasPrefix(resolved, "../") || strings.HasPrefix(resolved, "/") || windowsDrivePattern.MatchString(resolved) {
		return "", fmt.Errorf("import %q escapes the archive", specifier)
	}
	return resolved, nil
}

// namedExports runs top-level code in a temporary runtime and reads named exports.
// The runtime is not retained.
func (p *pluginProgram) namedExports() (map[string]ExportFunc, error) {
	if p == nil || p.module == nil {
		return nil, errors.New("plugin.js is required")
	}
	rt := moejs.NewRuntime(moejs.Options{
		DisableDynamicCode: true,
		Importer:           &moejs.Importer{Resolve: p.resolve},
	})
	var exports map[string]ExportFunc
	if err := runTopLevel(rt, func() error {
		if err := rt.Load(p.module); err != nil {
			return err
		}
		var err error
		exports, err = readNamedExports(rt, p.module)
		return err
	}); err != nil {
		return nil, err
	}
	return exports, nil
}

func runTopLevel(rt *moejs.Runtime, fn func() error) error {
	ctx, cancel := context.WithTimeout(context.Background(), pluginTopLevelTimeout)
	defer cancel()
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(done)
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error(context.Background(), "plugin.js timeout interrupt panicked", zap.Any("panic", recovered))
			}
		}()
		rt.Interrupt(errPluginTopLevelTimeout)
	})
	err := fn()
	if !stop() {
		<-done
	}
	if err == nil {
		return nil
	}
	if errors.Is(err, errPluginTopLevelTimeout) {
		return errPluginTopLevelTimeout
	}
	return err
}

func readNamedExports(rt *moejs.Runtime, mod *moejs.Module) (map[string]ExportFunc, error) {
	names := mod.Exports()
	out := make(map[string]ExportFunc, len(names))
	for _, name := range names {
		value, ok := rt.Export(name)
		if !ok {
			continue
		}
		exported, err := classifyExport(rt, value)
		if err != nil {
			return nil, fmt.Errorf("plugin.js export %s: %w", name, err)
		}
		out[name] = exported
	}
	return out, nil
}

func classifyExport(rt *moejs.Runtime, value moejs.Value) (ExportFunc, error) {
	if !value.IsObject() || !value.AsObject().IsCallable() {
		return ExportFunc{}, nil
	}
	async, err := isAsyncFunction(rt, value)
	if err != nil {
		return ExportFunc{}, err
	}
	return ExportFunc{Function: true, Async: async}, nil
}

// isAsyncFunction reports async functions and async generator functions.
// moejs does not expose an async-function predicate. The function's
// constructor name is AsyncFunction or AsyncGeneratorFunction.
func isAsyncFunction(rt *moejs.Runtime, fn moejs.Value) (bool, error) {
	ctor, err := rt.Get(fn, "constructor")
	if err != nil {
		return false, err
	}
	if !ctor.IsObject() {
		return false, nil
	}
	name, err := rt.Get(ctor, "name")
	if err != nil {
		return false, err
	}
	if !name.IsString() {
		return false, nil
	}
	switch name.String() {
	case "AsyncFunction", "AsyncGeneratorFunction":
		return true, nil
	default:
		return false, nil
	}
}

// compileContents links plugin.js when the package has one.
// UI-only packages return a nil module. The module is not executed.
func compileContents(contents *packageContents) (*moejs.Module, error) {
	if contents == nil || !contents.hasPluginJS() {
		if err := rejectDisallowed(contents, nil); err != nil {
			return nil, err
		}
		return nil, nil
	}
	prog, err := compilePluginSources(contents)
	if err != nil {
		return nil, err
	}
	if err := rejectDisallowed(contents, prog.closure()); err != nil {
		return nil, err
	}
	return prog.module, nil
}

// compilePackageModule compiles a ZIP that contains plugin.js.
// A package without plugin.js returns a nil module.
func compilePackageModule(data []byte) (*moejs.Module, error) {
	contents, err := readPackage(data)
	if err != nil {
		return nil, err
	}
	return compileContents(contents)
}

// evaluateSources compiles, links, and runs sources. Tests call it directly.
// sources uses archive paths. plugin.js is required.
func evaluateSources(sources map[string]string) (map[string]ExportFunc, error) {
	payloads := make(map[string][]byte, len(sources))
	for name, source := range sources {
		payloads[name] = []byte(source)
	}
	prog, err := compilePluginSources(&packageContents{sources: payloads})
	if err != nil {
		return nil, err
	}
	return prog.namedExports()
}
