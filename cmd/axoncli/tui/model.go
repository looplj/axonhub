package tui

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/bus"
	axonconf "github.com/looplj/axonhub/axon/conf"
	"github.com/looplj/axonhub/axon/permission/approval"
	"github.com/looplj/axonhub/axon/thread"
)

const (
	headerHeight              = 0
	statusBarHeight           = 1
	minTextareaHeight         = 3
	chromePadding             = 2
	inputBoxPadding           = 2 // Border top/bottom (1+1) + padding top/bottom (0+0) = 2
	inputBoxHorizontalPadding = 2 // Left border (1) + right border (1)
)

// thinkingBlock tracks the state of a single thinking block for display.
type thinkingBlock struct {
	content strings.Builder

	expanded bool // Whether the thinking content is expanded
	complete bool // Whether thinking is complete

	headerLineIndex int

	// Content lines are stored as multiple entries in Model.lines.
	// When collapsed, contentStartLineIndex == -1 and contentLineCount == 0.
	contentStartLineIndex int
	contentLineCount      int
}

// Model is the Bubbletea model for the AxonCode TUI.
type Model struct {
	viewport viewport.Model
	textarea textarea.Model
	spinner  spinner.Model

	lines      []string
	processing bool

	agentEvents <-chan agent.AgentEvent
	confEvents  <-chan axonconf.ReloadEvent
	agent       *agent.Agent
	reloadConf  func(context.Context) error
	bus         bus.EventBus
	threadID    string
	threadMgr   *thread.Manager

	ctx           context.Context
	cancel        context.CancelFunc
	processCancel context.CancelFunc

	model     string
	workspace string
	configDir string

	width  int
	height int
	ready  bool

	textareaHeight int

	lastCtrlC time.Time

	slashActive  bool
	slashMatches []slashCommand
	slashIndex   int
	slashOffset  int

	streamEvents            <-chan agent.AgentEvent
	streamText              *strings.Builder
	streamingStartLineIndex int
	streamingLineCount      int

	// Thinking blocks for collapsible thinking display
	thinkingBlocks []*thinkingBlock
	activeThinking *thinkingBlock

	// Mapping from viewport line index -> thinking block index (header only).
	// Rebuilt in syncViewport().
	thinkingHeaderViewportLine map[int]int

	// Model selector for switching models
	modelSelector *modelSelector

	// Text selection state for viewport content
	selecting      bool
	selectionStart selectionPos
	selectionEnd   selectionPos

	// Permission approval UI
	approvalSvc      approval.Service
	approvalReqCh    <-chan approval.Request
	approvalActive   bool
	approvalReq      approval.Request
	approvalSelector *approvalSelector

	// Logo display state
	showLogo bool
	logoTip  string
}

type selectionPos struct {
	line int
	col  int
}

// ModelOpts configures a new Model.
type ModelOpts struct {
	Agent      *agent.Agent
	Bus        bus.EventBus
	ThreadID   string
	ThreadMgr  *thread.Manager
	Ctx        context.Context
	Cancel     context.CancelFunc
	Model      string
	Workspace  string
	ConfigDir  string
	Approval   approval.Service
	ReloadConf func(context.Context) error
}

// NewModel creates a new TUI model.
func NewModel(opts ModelOpts) Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message... (Enter to send, Shift+Enter/Ctrl+J for newline)"
	ta.KeyMap = textarea.DefaultKeyMap()
	ta.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("shift+enter", "ctrl+j"),
		key.WithHelp("shift+enter", "insert newline"),
	)
	// Disable default up/down in textarea so we can handle them for line switching
	ta.KeyMap.LinePrevious = key.NewBinding(key.WithDisabled())
	ta.KeyMap.LineNext = key.NewBinding(key.WithDisabled())
	ta.Focus()
	ta.SetHeight(minTextareaHeight)
	ta.ShowLineNumbers = false

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Create a channel to receive agent events from the bus
	agentEvents := make(chan agent.AgentEvent, 256)
	confEvents := make(chan axonconf.ReloadEvent, 64)

	// Subscribe to agent events via bus
	opts.Bus.Subscribe(agent.TopicAgentEvent, bus.TypedHandler(func(_ context.Context, _ bus.Event, ev agent.AgentEvent) error {
		select {
		case agentEvents <- ev:
		default:
		}
		return nil
	}))
	opts.Bus.Subscribe(axonconf.TopicReloadEvent, bus.TypedHandler(func(_ context.Context, _ bus.Event, ev axonconf.ReloadEvent) error {
		select {
		case confEvents <- ev:
		default:
		}
		return nil
	}))

	m := Model{
		textarea:         ta,
		spinner:          sp,
		agent:            opts.Agent,
		reloadConf:       opts.ReloadConf,
		bus:              opts.Bus,
		agentEvents:      agentEvents,
		confEvents:       confEvents,
		threadID:         opts.ThreadID,
		threadMgr:        opts.ThreadMgr,
		ctx:              opts.Ctx,
		cancel:           opts.Cancel,
		model:            opts.Model,
		workspace:        opts.Workspace,
		configDir:        opts.ConfigDir,
		textareaHeight:   minTextareaHeight,
		streamText:       &strings.Builder{},
		approvalSvc:      opts.Approval,
		approvalSelector: newApprovalSelector(),
		showLogo:         true,
		logoTip:          getRandomTip(),
	}

	if opts.Approval != nil {
		m.approvalReqCh = opts.Approval.Subscribe(opts.Ctx)
	}

	// Initialize model selector
	m.modelSelector = newModelSelector(opts.ConfigDir, "", "", nil)

	return m
}

// Init returns the initial commands for the TUI.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		waitForAgentEvent(m.agentEvents),
		waitForConfEvent(m.confEvents),
		waitForApprovalRequest(m.approvalReqCh),
		m.spinner.Tick,
		textarea.Blink,
	)
}
