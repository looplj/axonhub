package main

import (
	"context"
	_ "embed"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/looplj/axonhub/cmd/axonhub-desktop/locales"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server"
)

const (
	panelWidth        = 400
	panelHeight       = 640
	panelGap          = 6
	panelRefreshEvery = 5 * time.Second
	panelLoadTimeout  = 15 * time.Second
	panelRadius       = 12
	copiedFor         = 1400 * time.Millisecond
)

const (
	panelRefreshError = "Couldn't refresh"
	panelOpenError    = "Couldn't open the page"
	panelBrowserError = "Couldn't open the browser"
)

// loadErrText is a panel error in the language the app shows. The stored
// value stays the English code, so a language change does not strand a
// comparison.
func loadErrText(err string) string {
	switch err {
	case panelRefreshError:
		return locales.CouldntRefresh()
	case panelOpenError:
		return locales.CouldntOpenPage()
	case panelBrowserError:
		return locales.CouldntOpenBrowser()
	default:
		return err
	}
}

// shell is the menu-bar window. Tray callbacks and the view read and
// change it on the main thread. session is the owner token and the Desktop
// project. snapshot is the last successful load; a failed load leaves it
// in place. sessionMu guards session while a refresh renews it.
//
// prefs is what settings chose (appearance and language), saved at
// prefsPath. secretPath is the owner password used to sign in again.
// configPath is the config.yml settings edits. offline means the server
// did not start (startErr says why): the panel stays on settings and loads
// nothing.
//
// copied names the Copy button that reads Copied ("url" or "key") until
// copySeq moves on.
//
// refreshCancel stops the reload timer while the panel is hidden.
// loadSeq drops a finished load when a newer one has already started.
// The timer runs off the main thread and hands each tick back before
// reading panel state. Each load then runs off the main thread.
// snapshot, selectedID, loadErr, hasSnapshot, copied, and copySeq are
// touched on the main thread only.
type shell struct {
	origin     string
	session    desktopSession
	sessionMu  sync.Mutex
	secretPath string
	win        *mygo.Window
	tray       *mygo.Tray
	quitting   bool

	snapshot      panelSnapshot
	selectedID    string
	loadErr       string
	hasSnapshot   bool
	copied        string
	copySeq       uint64
	prefs         panelPrefs
	prefsPath     string
	configPath    string
	page          panelPage
	form          settingsForm
	offline       bool
	startErr      string
	serverStop    *serverStop
	refreshCancel context.CancelFunc
	loadSeq       atomic.Uint64
}

func newShell(origin string, session desktopSession) *shell {
	return &shell{
		origin:  strings.TrimRight(origin, "/"),
		session: session,
	}
}

// loadPanel reads one global snapshot for the panel to render.
func (s *shell) loadPanel(ctx context.Context) (panelSnapshot, error) {
	return loadPanelSnapshot(ctx, s.origin, s.session, nil, time.Now())
}

// currentSession renews the owner token when it is due, or signs in again.
func (s *shell) currentSession(ctx context.Context) (desktopSession, error) {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	session, err := renewDesktopSession(ctx, nil, s.origin, s.secretPath, s.session, time.Now())
	if err != nil {
		return desktopSession{}, err
	}
	s.session = session
	return session, nil
}

// openAIBaseURL is the base URL an OpenAI-compatible client uses. origin
// is the listen address the desktop session already uses.
func openAIBaseURL(origin string) string {
	return strings.TrimRight(origin, "/") + "/v1"
}

// dockPolicy is the macOS activation policy. Accessory while the panel is
// hidden, so the process stays a menu-bar app; regular while it is
// visible, so the panel can take focus.
func dockPolicy(windowVisible bool) mygo.ActivationPolicy {
	if windowVisible {
		return mygo.ActivationPolicyRegular
	}
	return mygo.ActivationPolicyAccessory
}

func useMenuBarPolicy(windowVisible bool) {
	if runtime.GOOS != "darwin" {
		return
	}
	mygo.App.SetActivationPolicy(dockPolicy(windowVisible))
}

// panelAnchor places the panel next to the tray icon, inside the work
// area. A menu-bar icon sits at the top, so the panel opens beneath it; a
// bottom taskbar puts the panel above the icon instead. The whole panel
// stays inside the work area, including at the edge of a secondary
// display or beside a vertical taskbar. A zero-size anchor means the icon
// reported no bounds (AppIndicator, or a Windows overflow icon) and the
// caller centers the panel.
func panelAnchor(anchor, panel, work mygo.Rectangle) (x, y int, center bool) {
	if anchor.Width == 0 || anchor.Height == 0 {
		return 0, 0, true
	}
	x = anchor.X + (anchor.Width-panel.Width)/2
	y = anchor.Y + anchor.Height + panelGap
	if y+panel.Height > work.Y+work.Height {
		y = anchor.Y - panel.Height - panelGap
	}
	x = max(work.X, min(x, work.X+max(0, work.Width-panel.Width)))
	y = max(work.Y, min(y, work.Y+max(0, work.Height-panel.Height)))
	return x, y, false
}

func (s *shell) show() {
	useMenuBarPolicy(true)
	anchor := s.tray.Bounds()
	panel := s.win.Bounds()
	var area mygo.Rectangle
	if anchor.Width != 0 && anchor.Height != 0 {
		area = mygo.Screen.DisplayNearestPoint(mygo.Point{
			X: anchor.X + anchor.Width/2,
			Y: anchor.Y + anchor.Height/2,
		}).WorkArea
	}
	if x, y, center := panelAnchor(anchor, panel, area); center {
		s.win.Center()
	} else {
		s.win.SetPosition(x, y)
	}
	s.win.Show()
	s.win.Focus()
	s.startRefresh()
	s.loadPanelAsync()
}

func (s *shell) hide() {
	s.stopRefresh()
	if s.win != nil {
		s.win.Hide()
	}
	useMenuBarPolicy(false)
}

// startRefresh reloads the panel about every 5 seconds while it is visible.
func (s *shell) startRefresh() {
	if s.refreshCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.refreshCancel = cancel
	go func() {
		defer recoverLog("panel refresh")
		ticker := time.NewTicker(panelRefreshEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.win.Update(func() { s.loadPanelAsync() })
			}
		}
	}()
}

func (s *shell) stopRefresh() {
	if s.refreshCancel == nil {
		return
	}
	s.refreshCancel()
	s.refreshCancel = nil
}

// loadPanelAsync reads the snapshot off the main thread and applies it
// there. A newer load supersedes an older one. The window stays responsive.
func (s *shell) loadPanelAsync() {
	if s.win == nil || s.offline {
		return
	}
	seq := s.loadSeq.Add(1)
	origin := s.origin
	go func() {
		defer recoverLog("panel load")
		ctx, cancel := context.WithTimeout(context.Background(), panelLoadTimeout)
		defer cancel()
		session, err := s.currentSession(ctx)
		var snapshot panelSnapshot
		if err == nil {
			snapshot, err = loadPanelSnapshot(ctx, origin, session, nil, time.Now())
		}
		s.win.Update(func() { s.applySnapshot(seq, snapshot, err) })
	}()
}

func (s *shell) applySnapshot(seq uint64, snapshot panelSnapshot, err error) {
	if seq != s.loadSeq.Load() {
		return
	}
	if err != nil {
		log.Warn(context.Background(), "desktop panel refresh failed", log.Cause(err))
		s.loadErr = panelRefreshError
		return
	}
	s.snapshot = snapshot
	s.selectedID = nextSelectedID(snapshot.Requests, s.selectedID)
	s.hasSnapshot = true
	s.loadErr = ""
}

func (s *shell) copyBaseURL() {
	mygo.Clipboard.WriteText(openAIBaseURL(s.origin))
	s.flashCopied("url")
}

// copyDesktopKey writes the stored Desktop secret. An empty value means
// the key is not ready, so the click does nothing.
func (s *shell) copyDesktopKey(secret string) {
	if secret == "" {
		return
	}
	mygo.Clipboard.WriteText(secret)
	s.flashCopied("key")
}

// flashCopied makes one Copy button read Copied for a moment. A later
// copy restarts the moment.
func (s *shell) flashCopied(what string) {
	s.copied = what
	s.copySeq++
	seq := s.copySeq
	time.AfterFunc(copiedFor, func() {
		s.win.Update(func() {
			if s.copySeq == seq {
				s.copied = ""
			}
		})
	})
}

// openNext mints a one-time login path for projectID and opens it on the
// loopback origin. A mint failure leaves the panel as it is and does not
// open a URL that contains the owner token. The token is renewed first, so
// an expired desktop session is not written into the browser.
func (s *shell) openNext(projectID, nextPath string) {
	s.openNextFor(func(desktopSession) string { return projectID }, nextPath)
}

// openNextFor renews the session and mints the login off the main thread,
// since a renewal can wait on the network and on sessionMu, then opens the
// page there. project picks the project from the renewed session.
func (s *shell) openNextFor(project func(desktopSession) string, nextPath string) {
	if s.win == nil || s.offline {
		return
	}
	go func() {
		defer recoverLog("panel open")
		ctx, cancel := context.WithTimeout(context.Background(), panelLoadTimeout)
		defer cancel()
		path, errMsg := s.mintNext(ctx, project, nextPath)
		s.win.Update(func() { s.applyOpen(path, errMsg) })
	}()
}

// mintNext returns the login path to open, or the panel error to show.
func (s *shell) mintNext(ctx context.Context, project func(desktopSession) string, nextPath string) (string, string) {
	session, err := s.currentSession(ctx)
	if err != nil {
		log.Warn(context.Background(), "desktop session renewal failed", log.Cause(err))
		return "", panelOpenError
	}
	projectID := project(session)
	if projectID == "" {
		return "", panelOpenError
	}
	path, err := server.MintDesktopLogin(session.Token, projectID, nextPath)
	if err != nil || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		if err != nil {
			log.Warn(context.Background(), "desktop login handoff failed", log.Cause(err))
		}
		return "", panelOpenError
	}
	return path, ""
}

func (s *shell) applyOpen(path, errMsg string) {
	if s.offline {
		return
	}
	if errMsg != "" {
		s.loadErr = errMsg
		return
	}
	if err := mygo.Shell.OpenExternal(s.origin + path); err != nil {
		// The opener's error quotes the URL, and that URL carries the
		// one-time login code, so the log stays a fixed message.
		log.Warn(context.Background(), "open desktop page failed")
		s.loadErr = panelBrowserError
		return
	}
	if s.loadErr == panelOpenError || s.loadErr == panelBrowserError {
		s.loadErr = ""
	}
}

// setTheme switches the panel's appearance and saves it.
func (s *shell) setTheme(mode themeMode) {
	s.prefs.Theme = mode
	s.savePrefs()
}

// savePrefs writes prefs. A failed save keeps them for this run.
func (s *shell) savePrefs() {
	if err := writePanelPrefs(s.prefsPath, s.prefs); err != nil {
		log.Warn(context.Background(), "desktop panel settings not saved", log.Cause(err))
	}
}

func (s *shell) openAdmin() {
	s.openNextFor(func(session desktopSession) string { return session.ProjectID }, adminNextPath)
}

func (s *shell) chooseRequest(request panelRequest) {
	s.selectedID = request.ID
	s.openNext(request.ProjectID, requestAdminPath(request.ID))
}

func (s *shell) toggle() {
	if s.win != nil && s.win.IsVisible() {
		s.hide()
		return
	}
	s.show()
}

func (s *shell) ready() {
	s.applyLanguage()
	var err error
	s.tray, err = mygo.NewTray(mygo.TrayOptions{
		Icon:           trayIcon(),
		IconIsTemplate: runtime.GOOS == "darwin",
		ToolTip:        "AxonHub",
	})
	if err != nil {
		fatal(fmt.Errorf("create tray icon: %w", err))
	}
	s.win = mygo.NewWindow(mygo.WindowOptions{
		Title:             "AxonHub",
		Width:             panelWidth,
		Height:            panelHeight,
		Hidden:            true,
		Frameless:         true,
		DisableResize:     true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		AlwaysOnTop:       true,
		SkipTaskbar:       true,
		CornerRadius:      panelRadius,
		BackgroundColor:   panelBackgroundColor(s.prefs.Theme),
		Content:           ui.View(s.view),
	})
	s.win.OnBlur(s.hide)
	s.win.OnClose(func(e *mygo.CloseEvent) {
		// Close, and the shortcuts that close a window, hide the panel.
		// App.Quit sets quitting first so that close can finish.
		if !s.quitting {
			e.PreventDefault()
			s.hide()
		}
	})

	if trayMenuStays() {
		// AppIndicator has no primary click. Its first item opens the
		// same panel a status-item click toggles on macOS and Windows.
		s.tray.SetMenu(s.trayMenu())
	} else {
		// TrayOptions.Menu would consume the primary click on macOS.
		// Leave it unset so the icon toggles the panel. The menu is
		// installed for the right-click popup only, then cleared so the
		// next primary click still toggles.
		s.tray.OnClick(s.toggle)
		s.tray.OnRightClick(func() {
			s.hide()
			s.tray.SetMenu(s.trayMenu())
			defer s.tray.SetMenu(nil)
			s.tray.PopUpMenu()
		})
	}
	s.watchChrome()
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		if s.tray != nil {
			s.tray.Destroy()
		}
		// Windows are already closed. Stop the embedded server before the
		// process exits, so open requests can finish and the database closes.
		if s.serverStop != nil {
			s.serverStop.stop()
		}
	})
}

// trayTemplate is the synapse mark, black on transparent, so macOS can tint it.
//
//go:embed resources/tray-template.png
var trayTemplate []byte

// trayColor is the same mark in the app colors, for notification areas that
// do not tint a template.
//
//go:embed resources/tray.png
var trayColor []byte

func trayIcon() []byte {
	if runtime.GOOS == "darwin" {
		return trayTemplate
	}
	return trayColor
}
