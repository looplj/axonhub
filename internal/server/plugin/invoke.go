package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/MoeForge/moejs"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

const (
	vmPoolSize          = 16
	circuitFailureLimit = 20
	circuitPause        = 5 * time.Minute
	pluginStateBytes    = 64 << 10
)

var (
	// ErrVMUnavailable means the pool could not lend a runtime before ctx ended.
	// It is overload, not a call failure, and does not open the circuit breaker.
	ErrVMUnavailable = errors.New("plugin vm unavailable")

	// ErrCircuitOpen means this process has paused the plugin after consecutive call failures.
	ErrCircuitOpen = errors.New("plugin circuit open")
)

// InvokeRequest is one call into an installed plugin's named export.
// Args is a JSON array of values placed before ctx. Nil, empty, or JSON null
// calls the export with only ctx. State nil or JSON null starts at {}.
// Config nil starts at {}. Timeout bounds JavaScript after a runtime is
// borrowed and does not include time spent waiting for one. Zero uses ctx
// as the only script deadline.
//
// Generation, when set, is the snapshot captured for this request. The call
// uses it even if a newer generation has been published. BorrowWait limits
// how long to wait for a runtime. Zero waits until ctx ends. The wait is not
// part of Timeout. ValidateReturn runs after a successful script return and
// before the success counter. A non-nil error is a call failure.
// HostRequest and HostAttempt are optional JSON objects placed on ctx.
// AllowReject adds ctx.reject for onRequest.
type InvokeRequest struct {
	InstallationID int
	Export         string
	Args           json.RawMessage
	State          json.RawMessage
	Config         map[string]any
	Timeout        time.Duration
	Generation     *Generation
	BorrowWait     time.Duration
	ValidateReturn func(json.RawMessage) error
	HostRequest    json.RawMessage
	HostAttempt    json.RawMessage
	AllowReject    bool
}

// InvokeResult is a successful call.
// Value is omitted when the export returns undefined.
// State is the ctx.state to commit.
type InvokeResult struct {
	Value json.RawMessage
	State json.RawMessage
}

// Invoke borrows a runtime for one call and returns it unless the runtime was interrupted.
// The linked module stored for the installation is loaded on the first use of each
// runtime. Later calls on that runtime do not load it again.
func (m *Manager) Invoke(ctx context.Context, req InvokeRequest) (InvokeResult, error) {
	if m == nil {
		return InvokeResult{}, errors.New("plugin manager is nil")
	}
	if ctx == nil {
		return InvokeResult{}, errors.New("plugin invoke requires a context")
	}
	if req.InstallationID <= 0 {
		return InvokeResult{}, errors.New("plugin installation id must be positive")
	}
	if req.Export == "" {
		return InvokeResult{}, errors.New("plugin export is required")
	}
	if err := validateInvokeArgs(req.Args); err != nil {
		return InvokeResult{}, err
	}
	state, err := normalizeInvokeState(req.State)
	if err != nil {
		return InvokeResult{}, err
	}
	config, err := configJSON(req.Config)
	if err != nil {
		return InvokeResult{}, err
	}

	gen := req.Generation
	pinned := gen != nil
	if gen == nil {
		gen = m.current()
	}
	if gen == nil {
		return InvokeResult{}, ErrGenerationClosed
	}
	var acquireErr error
	if pinned {
		acquireErr = gen.acquirePinned()
	} else {
		acquireErr = gen.Acquire()
	}
	if acquireErr != nil {
		return InvokeResult{}, acquireErr
	}
	defer gen.Release()

	loaded := gen.ByID(req.InstallationID)
	if loaded == nil || loaded.Status != RuntimeStatusActive || loaded.resource == nil {
		return InvokeResult{}, fmt.Errorf("plugin %d is not active", req.InstallationID)
	}
	module := loaded.resource.compiledModule()
	if module == nil {
		return InvokeResult{}, fmt.Errorf("plugin %d has no JavaScript module", req.InstallationID)
	}
	if !slices.Contains(module.Exports(), req.Export) {
		return InvokeResult{}, fmt.Errorf("plugin export %q was not found", req.Export)
	}

	exec := m.execFor(req.InstallationID)
	if exec.rejectCall(m.nowCall()) {
		return InvokeResult{}, ErrCircuitOpen
	}
	borrowCtx := ctx
	cancelBorrow := func() {}
	if req.BorrowWait > 0 {
		var cancel context.CancelFunc
		borrowCtx, cancel = context.WithTimeout(ctx, req.BorrowWait)
		cancelBorrow = cancel
	}
	vm, err := exec.borrow(borrowCtx, module)
	cancelBorrow()
	if err != nil {
		return InvokeResult{}, err
	}
	// Discard unless callExport reports that the runtime is safe to reuse.
	// A panic leaves the initial value in place, so the runtime is not returned.
	discard := true
	defer func() { exec.release(vm, discard) }()

	result, callErr, discard := m.callExport(ctx, vm, callExportInput{
		pluginID:       loaded.PluginID,
		installationID: req.InstallationID,
		export:         req.Export,
		args:           req.Args,
		state:          state,
		config:         config,
		timeout:        req.Timeout,
		granted:        loaded.GrantedPermissions,
		network:        loaded.GrantedNetwork,
		requestJSON:    req.HostRequest,
		attemptJSON:    req.HostAttempt,
		allowReject:    req.AllowReject,
	})
	if callErr != nil {
		if countsAsCallFailure(callErr) {
			exec.recordFailure(m.nowCall())
		}
		return InvokeResult{}, callErr
	}
	if req.ValidateReturn != nil {
		if err := req.ValidateReturn(result.Value); err != nil {
			if !errors.Is(err, errProtectedRewrite) {
				exec.recordFailure(m.nowCall())
			}
			return InvokeResult{}, err
		}
	}
	exec.recordSuccess()
	return result, nil
}

type callExportInput struct {
	pluginID       string
	installationID int
	export         string
	args           json.RawMessage
	state          json.RawMessage
	config         json.RawMessage
	timeout        time.Duration
	granted        []string
	network        []string
	requestJSON    json.RawMessage
	attemptJSON    json.RawMessage
	allowReject    bool
}

func (m *Manager) callExport(ctx context.Context, vm *pluginVM, in callExportInput) (InvokeResult, error, bool) {
	callCtx, disarm := armPluginInterrupt(vm, ctx, in.timeout)
	// ioCtx is canceled only after the call is marked finished. A timeout must
	// not complete in-flight HTTP early enough for its callback to run.
	ioCtx, cancelIO := context.WithCancel(context.WithoutCancel(callCtx))
	host := &hostCall{
		manager:        m,
		rt:             vm.rt,
		ctx:            callCtx,
		ioCtx:          ioCtx,
		cancelIO:       cancelIO,
		pluginID:       in.pluginID,
		installationID: in.installationID,
		export:         in.export,
		alive:          true,
		granted:        permissionSet(in.granted),
		network:        append([]string(nil), in.network...),
		async:          newCallAsync(),
	}
	defer cancelIO()
	defer func() { host.alive = false }()

	discard, callErr := host.run(vm, in)
	fired, cause := disarm()
	if fired {
		discard = true
		if host.rejected == nil && (callErr == nil || isInterrupt(callErr)) {
			if cause == nil {
				cause = errors.New("plugin call interrupted")
			}
			callErr = fmt.Errorf("plugin call failed: %w", cause)
		}
	}
	// A caught or uncaught ctx.reject still rejects. It is not a call failure.
	if host.rejected != nil {
		return InvokeResult{}, host.rejected, discard
	}
	if callErr != nil {
		return InvokeResult{}, callErr, discard
	}
	return host.result, nil, discard
}

func (h *hostCall) run(vm *pluginVM, in callExportInput) (discard bool, err error) {
	if err := vm.ensureLoaded(); err != nil {
		return true, callFailed(err)
	}
	ctxValue, err := h.build(in.config, in.state, in.requestJSON, in.attemptJSON, in.allowReject)
	if err != nil {
		return isFatalRuntime(err), callFailed(err)
	}
	h.ctxValue = ctxValue
	args, err := h.scriptArgs(in.args)
	if err != nil {
		return isFatalRuntime(err), callFailed(err)
	}
	args = append([]moejs.Value{ctxValue}, args...)
	fn, ok := vm.rt.Export(in.export)
	if !ok || !fn.IsObject() || !fn.AsObject().IsCallable() {
		return false, fmt.Errorf("plugin export %q is not a function", in.export)
	}
	returned, err := vm.rt.CallFunction(fn, moejs.Undefined(), args...)
	if err != nil {
		return h.end(isFatalRuntime(err), callFailed(err))
	}
	for {
		state, result, isPromise := moejs.PromiseResult(returned)
		if !isPromise {
			return h.accept(returned)
		}
		switch state {
		case moejs.PromiseFulfilled:
			return h.accept(result)
		case moejs.PromiseRejected:
			return h.end(false, h.promiseFailure(result))
		}
		callback, waitErr := h.async.next(h.ctx)
		if waitErr != nil {
			return h.end(true, callFailed(waitErr))
		}
		if err := callback(); err != nil {
			return h.end(isFatalRuntime(err), callFailed(err))
		}
	}
}

func (h *hostCall) accept(value moejs.Value) (bool, error) {
	raw, err := h.readReturn(value)
	if err != nil {
		return h.end(isFatalRuntime(err), callFailed(err))
	}
	state, err := h.readState()
	if err != nil {
		return h.end(isFatalRuntime(err), callFailed(err))
	}
	h.result = InvokeResult{Value: raw, State: state}
	return h.end(false, nil)
}

// end closes result delivery and cancels in-flight host I/O.
// A call that still has tasks or queued callbacks discards its VM.
func (h *hostCall) end(discard bool, err error) (bool, error) {
	if h.async != nil && h.async.close() {
		discard = true
	}
	if h.cancelIO != nil {
		h.cancelIO()
	}
	return discard, err
}

func (h *hostCall) promiseFailure(reason moejs.Value) error {
	message := "promise rejected"
	if reason.IsString() {
		message = reason.String()
	} else if reason.IsObject() {
		if msg, err := h.rt.Get(reason, "message"); err == nil && msg.IsString() && msg.String() != "" {
			message = msg.String()
		}
	}
	return fmt.Errorf("plugin call failed: %s", message)
}

// callAsync is one hook call's host tasks and the callbacks that may touch its VM.
// HTTP workers only enqueue. The VM goroutine runs the callbacks.
type callAsync struct {
	mu       sync.Mutex
	finished bool
	tasks    int
	queue    []func() error
	wake     chan struct{}
}

func newCallAsync() *callAsync {
	return &callAsync{wake: make(chan struct{}, 1)}
}

func (a *callAsync) track() bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.finished {
		return false
	}
	a.tasks++
	return true
}

// deliver queues fn for the VM goroutine. A finished call drops fn and returns false.
func (a *callAsync) deliver(fn func() error) bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.tasks > 0 {
		a.tasks--
	}
	if a.finished || fn == nil {
		return false
	}
	a.queue = append(a.queue, fn)
	select {
	case a.wake <- struct{}{}:
	default:
	}
	return true
}

func (a *callAsync) next(ctx context.Context) (func() error, error) {
	for {
		a.mu.Lock()
		if len(a.queue) > 0 {
			fn := a.queue[0]
			a.queue = a.queue[1:]
			a.mu.Unlock()
			return fn, nil
		}
		a.mu.Unlock()
		if ctx == nil {
			ctx = context.Background()
		}
		select {
		case <-ctx.Done():
			err := context.Cause(ctx)
			if err == nil {
				err = ctx.Err()
			}
			return nil, err
		case <-a.wake:
		}
	}
}

// close stops delivery. It reports whether tasks or callbacks were still outstanding.
func (a *callAsync) close() bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.finished = true
	pending := a.tasks > 0 || len(a.queue) > 0
	a.queue = nil
	return pending
}

func callFailed(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*moejs.InterruptedError](err); ok {
		return fmt.Errorf("plugin call failed: %w", err)
	}
	if exception, ok := errors.AsType[*moejs.Exception](err); ok {
		message := exception.Message()
		if message == "" {
			message = exception.Error()
		}
		return fmt.Errorf("plugin call failed: %s", message)
	}
	return fmt.Errorf("plugin call failed: %w", err)
}

func countsAsCallFailure(err error) bool {
	if err == nil ||
		xerrors.Is(err, ErrVMUnavailable) ||
		xerrors.Is(err, ErrCircuitOpen) ||
		xerrors.Is(err, ErrGenerationClosed) ||
		xerrors.Is(err, context.Canceled) {
		return false
	}
	_, rejected := errors.AsType[*requestRejectedError](err)
	return !rejected
}

// ConsecutiveFailures is the process-local circuit streak for one installation.
// Overload does not increment it. Zero means the plugin is not in a failure streak.
func (m *Manager) ConsecutiveFailures(installationID int) int {
	if m == nil || installationID <= 0 {
		return 0
	}
	m.execMu.Lock()
	exec := m.execs[installationID]
	m.execMu.Unlock()
	if exec == nil {
		return 0
	}
	exec.mu.Lock()
	defer exec.mu.Unlock()
	return exec.failures
}

func isInterrupt(err error) bool {
	var interrupted *moejs.InterruptedError
	return errors.As(err, &interrupted)
}

func isFatalRuntime(err error) bool {
	if isInterrupt(err) {
		return true
	}
	var internal *moejs.InternalError
	return errors.As(err, &internal)
}

func validateInvokeArgs(raw json.RawMessage) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(trimmed, &elems); err != nil {
		return errors.New("plugin invoke args must be a JSON array")
	}
	return nil
}

func normalizeInvokeState(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return json.RawMessage(`{}`), nil
	}
	if !json.Valid(trimmed) || trimmed[0] != '{' {
		return nil, errors.New("plugin state must be a JSON object")
	}
	if len(trimmed) > pluginStateBytes {
		return nil, errors.New("plugin state exceeds 64 KiB")
	}
	return append(json.RawMessage(nil), trimmed...), nil
}

func configJSON(config map[string]any) (json.RawMessage, error) {
	if config == nil {
		return json.RawMessage(`{}`), nil
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("plugin config cannot be encoded: %w", err)
	}
	if len(raw) > MaxConfigBytes {
		return nil, errors.New("plugin config exceeds 64 KiB")
	}
	if len(raw) == 0 || raw[0] != '{' {
		return nil, errors.New("plugin config must be an object")
	}
	return raw, nil
}

func (m *Manager) nowCall() time.Time {
	if m != nil && m.callNow != nil {
		return m.callNow()
	}
	return time.Now()
}

func (m *Manager) execFor(installationID int) *pluginExec {
	m.execMu.Lock()
	defer m.execMu.Unlock()
	if m.execs == nil {
		m.execs = map[int]*pluginExec{}
	}
	exec := m.execs[installationID]
	if exec == nil {
		exec = &pluginExec{}
		m.execs[installationID] = exec
	}
	return exec
}

// releaseExecModule runs after the last generation using a resource has drained.
// Keep a newer module's pool and preserve the breaker across upgrades.
func (m *Manager) releaseExecModule(id int, module *moejs.Module) {
	m.execMu.Lock()
	defer m.execMu.Unlock()
	exec := m.execs[id]
	if exec == nil {
		return
	}
	exec.mu.Lock()
	defer exec.mu.Unlock()
	if exec.module == module {
		exec.idle = nil
		exec.module = nil
	}
	loaded := m.current().ByID(id)
	if exec.borrowed == 0 && (loaded == nil || loaded.Status != RuntimeStatusActive) {
		delete(m.execs, id)
	}
}

func (m *Manager) closeExecs() {
	if m == nil {
		return
	}
	m.execMu.Lock()
	execs := m.execs
	m.execs = map[int]*pluginExec{}
	m.execMu.Unlock()
	for _, exec := range execs {
		exec.mu.Lock()
		exec.idle = nil
		exec.module = nil
		exec.mu.Unlock()
	}
}

// pluginExec is one installation's runtime pool and circuit breaker.
// The breaker stays with the installation when the linked module changes.
type pluginExec struct {
	mu        sync.Mutex
	idle      []*pluginVM
	borrowed  int
	module    *moejs.Module
	waiters   []chan struct{}
	failures  int
	openUntil time.Time
}

func (e *pluginExec) rejectCall(now time.Time) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.openUntil.IsZero() {
		return false
	}
	if now.Before(e.openUntil) {
		return true
	}
	e.openUntil = time.Time{}
	e.failures = 0
	return false
}

func (e *pluginExec) recordFailure(now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.openUntil.IsZero() && now.Before(e.openUntil) {
		return
	}
	e.failures++
	if e.failures >= circuitFailureLimit {
		e.openUntil = now.Add(circuitPause)
		e.failures = 0
	}
}

func (e *pluginExec) recordSuccess() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failures = 0
	e.openUntil = time.Time{}
}

func (e *pluginExec) borrow(ctx context.Context, module *moejs.Module) (*pluginVM, error) {
	e.mu.Lock()
	for {
		if ctx.Err() != nil {
			// A canceled waiter may have consumed a wakeup without borrowing.
			// Pass the available capacity on so other waiters do not stay asleep.
			if len(e.idle) > 0 || e.borrowed < vmPoolSize {
				e.wake()
			}
			e.mu.Unlock()
			return nil, ErrVMUnavailable
		}
		if e.module != module {
			e.idle = nil
			e.module = module
		}
		if n := len(e.idle); n > 0 {
			vm := e.idle[n-1]
			e.idle = e.idle[:n-1]
			e.borrowed++
			e.mu.Unlock()
			return vm, nil
		}
		if e.borrowed < vmPoolSize {
			e.borrowed++
			vm := newPluginVM(module)
			e.mu.Unlock()
			return vm, nil
		}
		ready := make(chan struct{}, 1)
		e.waiters = append(e.waiters, ready)
		e.mu.Unlock()

		select {
		case <-ctx.Done():
			e.mu.Lock()
			e.removeWaiter(ready)
		case <-ready:
			e.mu.Lock()
		}
	}
}

func (e *pluginExec) release(vm *pluginVM, discard bool) {
	if vm == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.borrowed > 0 {
		e.borrowed--
	}
	if !discard && vm.module == e.module {
		vm.rt.ClearInterrupt()
		vm.rt.ReleaseCallData()
		e.idle = append(e.idle, vm)
	}
	e.wake()
}

func (e *pluginExec) wake() {
	if len(e.waiters) == 0 {
		return
	}
	ready := e.waiters[0]
	e.waiters = e.waiters[1:]
	select {
	case ready <- struct{}{}:
	default:
	}
}

func (e *pluginExec) removeWaiter(ready chan struct{}) {
	for i, waiter := range e.waiters {
		if waiter == ready {
			e.waiters = slices.Delete(e.waiters, i, i+1)
			return
		}
	}
}

type pluginVM struct {
	rt     *moejs.Runtime
	module *moejs.Module
	loaded bool
}

func newPluginVM(module *moejs.Module) *pluginVM {
	return &pluginVM{
		rt: moejs.NewRuntime(moejs.Options{
			DisableDynamicCode: true,
		}),
		module: module,
	}
}

func (vm *pluginVM) ensureLoaded() error {
	if vm == nil || vm.rt == nil {
		return errors.New("plugin runtime is missing")
	}
	if vm.loaded {
		return nil
	}
	if err := vm.rt.Load(vm.module); err != nil {
		return err
	}
	vm.loaded = true
	return nil
}

// armPluginInterrupt stops JavaScript when the script budget ends.
// A positive timeout starts when this is called, after a runtime has been
// borrowed, and is not shortened by a deadline already set on parent.
// Parent cancellation still stops the call. A parent deadline does not.
// A zero timeout uses parent itself, so its deadline is the script budget.
// The returned function reports whether Interrupt ran.
func armPluginInterrupt(vm *pluginVM, parent context.Context, timeout time.Duration) (context.Context, func() (bool, error)) {
	if timeout <= 0 {
		return armParentDeadline(vm, parent)
	}
	ctx, cancel := context.WithCancelCause(context.WithoutCancel(parent))
	timer := time.AfterFunc(timeout, func() {
		cancel(context.DeadlineExceeded)
	})
	var stopParent func() bool
	if parent != nil && parent.Done() != nil {
		stopParent = context.AfterFunc(parent, func() {
			err := parent.Err()
			if err != nil && !xerrors.Is(err, context.DeadlineExceeded) {
				cancel(err)
			}
		})
		if err := parent.Err(); err != nil && !xerrors.Is(err, context.DeadlineExceeded) {
			cancel(err)
		}
	}
	stopCall := interruptOnDone(vm, ctx)
	return ctx, func() (bool, error) {
		timer.Stop()
		if stopParent != nil {
			stopParent()
		}
		fired := !stopCall()
		if !fired {
			return false, nil
		}
		cause := context.Cause(ctx)
		if cause == nil {
			cause = ctx.Err()
		}
		return true, cause
	}
}

func armParentDeadline(vm *pluginVM, parent context.Context) (context.Context, func() (bool, error)) {
	if parent == nil {
		parent = context.Background()
	}
	if parent.Done() == nil {
		return parent, func() (bool, error) { return false, nil }
	}
	stop := interruptOnDone(vm, parent)
	return parent, func() (bool, error) {
		if stop() {
			return false, nil
		}
		cause := context.Cause(parent)
		if cause == nil {
			cause = parent.Err()
		}
		return true, cause
	}
}

func interruptOnDone(vm *pluginVM, ctx context.Context) func() bool {
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(done)
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error(context.Background(), "plugin call interrupt panicked",
					log.Any("panic", recovered),
					log.Stack("stack"))
			}
		}()
		cause := context.Cause(ctx)
		if cause == nil {
			cause = ctx.Err()
		}
		vm.rt.Interrupt(cause)
	})
	return func() bool {
		if stop() {
			return true
		}
		<-done
		return false
	}
}
