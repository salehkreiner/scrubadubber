// Command scrubadubber is the always-on tray (Windows) / menubar (macOS) app.
// It supervises the Scrubadubber Hub, surfaces protection status as an icon
// color, hosts a loopback settings page, and self-updates.
//
// It contains no scrubbing logic — that lives in the Hub binary it manages.
package main

import (
	"context"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"fyne.io/systray"

	"github.com/salehkreiner/scrubadubber/assets"
	"github.com/salehkreiner/scrubadubber/internal/bridgecheck"
	"github.com/salehkreiner/scrubadubber/internal/config"
	"github.com/salehkreiner/scrubadubber/internal/hubmanager"
	"github.com/salehkreiner/scrubadubber/internal/logfile"
	"github.com/salehkreiner/scrubadubber/internal/provision"
	"github.com/salehkreiner/scrubadubber/internal/settings"
	"github.com/salehkreiner/scrubadubber/internal/startup"
	"github.com/salehkreiner/scrubadubber/internal/sysopen"
	"github.com/salehkreiner/scrubadubber/internal/updater"
	"github.com/salehkreiner/scrubadubber/internal/version"
)

type app struct {
	ctx    context.Context
	cancel context.CancelFunc

	settingsPath string
	hubLogPath   string
	hubLog       *logfile.Writer

	hub        *hubmanager.Manager
	startupMgr startup.Manager
	server     *settings.Server
	updClient  *updater.Client

	lock net.Listener

	mu        sync.Mutex
	cur       settings.Settings
	hubState  hubmanager.State
	protected bool
	pending   *updater.Update

	mProtected *systray.MenuItem
	mStart     *systray.MenuItem
	mStop      *systray.MenuItem
	mRestart   *systray.MenuItem
	mLogs      *systray.MenuItem
	mSettings  *systray.MenuItem
	mUpdate    *systray.MenuItem
	mQuit      *systray.MenuItem
}

func main() {
	lock, ok := acquireLock()
	if !ok {
		// Another instance already holds the lock; nothing to do.
		return
	}

	a := &app{lock: lock}
	a.ctx, a.cancel = context.WithCancel(context.Background())
	a.settingsPath, _ = config.SettingsPath()
	a.hubLogPath, _ = config.HubLogPath()

	a.setupAppLog()
	log.Printf("%s %s starting", config.AppName, version.Version)

	// Clean up a post-update backup of ourselves, if any.
	if exe, err := os.Executable(); err == nil {
		updater.CleanupOldBinary(exe)
	}

	systray.Run(a.onReady, a.onExit)
}

// acquireLock binds a fixed loopback port to enforce single-instance.
func acquireLock() (net.Listener, bool) {
	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(config.LockPort))
	if err != nil {
		return nil, false
	}
	return ln, true
}

func (a *app) setupAppLog() {
	logDir, err := config.LogDir()
	if err != nil {
		return
	}
	w, err := logfile.Open(filepath.Join(logDir, "app.log"), 0, 0)
	if err != nil {
		return
	}
	log.SetOutput(w)
}

func (a *app) onReady() {
	systray.SetIcon(assets.Icon(assets.Grey))
	systray.SetTitle("") // icon-only in the menubar/tray
	systray.SetTooltip(config.AppName)

	cur, err := settings.Load(a.settingsPath)
	if err != nil {
		log.Printf("load settings: %v", err)
	}
	a.cur = cur

	if w, err := logfile.Open(a.hubLogPath, 0, 0); err == nil {
		a.hubLog = w
	}

	if exe, err := startup.ExecPath(); err == nil {
		a.startupMgr = startup.New(exe, "--startup")
		a.applyStartupSetting(cur.StartOnLogin)
	}

	a.updClient = updater.NewClient()
	a.server = settings.NewServer(settings.Options{
		Path:       a.settingsPath,
		Apply:      a.applySettings,
		OpenConfig: a.openHubConfig,
		Status:     a.status,
	})

	a.hub = a.buildHub(cur)

	// Build the menu before starting the Hub so state callbacks find the items.
	a.buildMenu()
	go a.handleClicks()

	a.hub.SetOnState(a.onHubState)
	a.hub.Run(a.ctx)
	a.hub.Start()

	// First run with no Hub binary present (the common case on macOS, where the
	// .app is the installer): provision in the background. The Hub supervisor
	// picks the binary up once it lands.
	a.maybeFirstRunProvision()

	(&updater.Poller{
		Client:   a.updClient,
		Current:  version.Version,
		OnUpdate: a.onUpdateAvailable,
	}).Run(a.ctx)
}

func (a *app) onExit() {
	a.cancel()
	if a.hub != nil {
		a.hub.Close()
	}
	if a.server != nil {
		_ = a.server.Close()
	}
	if a.hubLog != nil {
		_ = a.hubLog.Close()
	}
	if a.lock != nil {
		_ = a.lock.Close()
	}
}

// maybeFirstRunProvision downloads the Hub/bridge in the background if the Hub
// binary isn't present yet. The app binary is already in place (we're running
// it), so AppVersion is left empty.
func (a *app) maybeFirstRunProvision() {
	hubPath, err := config.HubPath()
	if err != nil {
		return
	}
	if _, err := os.Stat(hubPath); err == nil {
		return // already installed
	}
	go func() {
		log.Printf("first run: provisioning Hub and bridge…")
		opts := provision.Options{
			HubVersion:    config.PinnedHubVersion,
			BridgeVersion: config.PinnedBridgeVersion,
			Log:           func(f string, args ...any) { log.Printf(f, args...) },
		}
		if exe, err := os.Executable(); err == nil && a.cur.StartOnLogin {
			opts.StartupTarget = exe
		}
		if err := provision.Install(a.ctx, opts); err != nil {
			log.Printf("first-run provision failed: %v", err)
			return
		}
		a.hub.Start() // reset backoff now that the binary exists
	}()
}

func (a *app) buildHub(s settings.Settings) *hubmanager.Manager {
	hubPath, _ := config.HubPath()
	cfgPath, _ := config.HubConfigPath()
	dataDir, _ := config.DataDir()
	// Scrubbing mode is driven by the Settings dropdown via the Hub's
	// SCRUB_DEFAULT_MODE env override (applied after config.yaml, so it wins);
	// applySettings restarts the Hub when the mode changes so it takes effect.
	// Other scrubbing config stays in config.yaml ("Open config file").
	//
	// WorkDir anchors the Hub's relative paths (sqlite state, ./ca/...) under the
	// data dir instead of whatever CWD the tray inherited at login.
	return hubmanager.New(hubmanager.Config{
		HubPath:   hubPath,
		Args:      []string{"serve", "-config", cfgPath},
		Env:       []string{"SCRUB_DEFAULT_MODE=" + string(s.Mode)},
		WorkDir:   dataDir,
		HealthURL: config.HealthURLFor(s.HubURL),
		LogWriter: a.hubLog,
	})
}

func (a *app) buildMenu() {
	title := systray.AddMenuItem("● "+config.AppName, "")
	title.Disable()
	systray.AddSeparator()

	a.mProtected = systray.AddMenuItem("Starting…", "")
	a.mProtected.Disable()
	systray.AddSeparator()

	a.mStart = systray.AddMenuItem("Start Hub", "Start the Scrubadubber Hub")
	a.mStop = systray.AddMenuItem("Stop Hub", "Stop the Scrubadubber Hub")
	a.mRestart = systray.AddMenuItem("Restart Hub", "Restart the Scrubadubber Hub")
	systray.AddSeparator()

	a.mLogs = systray.AddMenuItem("View Logs...", "Open the Hub log file")
	a.mSettings = systray.AddMenuItem("Settings...", "Open Scrubadubber settings")
	systray.AddSeparator()

	about := systray.AddMenuItem("About "+config.AppName+" v"+version.Version, "")
	about.Disable()
	a.mUpdate = systray.AddMenuItem("Check for Updates", "Check for a newer version")
	systray.AddSeparator()

	a.mQuit = systray.AddMenuItem("Quit", "Quit "+config.AppName)

	a.refreshHubMenu(a.hub.State())
}

func (a *app) handleClicks() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.mStart.ClickedCh:
			a.hub.Start()
		case <-a.mStop.ClickedCh:
			a.hub.Stop()
		case <-a.mRestart.ClickedCh:
			a.hub.Restart()
		case <-a.mLogs.ClickedCh:
			if err := sysopen.Open(a.hubLogPath); err != nil {
				log.Printf("open logs: %v", err)
			}
		case <-a.mSettings.ClickedCh:
			a.openSettings()
		case <-a.mUpdate.ClickedCh:
			a.onUpdateClicked()
		case <-a.mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

// onHubState maps Hub status (+ bridge config) onto the icon, tooltip, status
// line, and Start/Stop visibility.
func (a *app) onHubState(state hubmanager.State) {
	protected := state == hubmanager.Healthy && bridgecheck.Configured()
	a.mu.Lock()
	a.hubState = state
	a.protected = protected
	a.mu.Unlock()

	icon, label := presentation(state, protected)
	systray.SetIcon(assets.Icon(icon))
	systray.SetTooltip(config.AppName + " — " + label)
	if a.mProtected != nil {
		a.mProtected.SetTitle(label)
	}
	a.refreshHubMenu(state)
}

func (a *app) refreshHubMenu(state hubmanager.State) {
	if a.mStart == nil || a.mStop == nil {
		return
	}
	if state == hubmanager.Down {
		a.mStart.Show()
		a.mStop.Hide()
	} else {
		a.mStart.Hide()
		a.mStop.Show()
	}
}

// presentation returns the icon and status-line label for a state.
func presentation(state hubmanager.State, protected bool) (assets.Status, string) {
	switch state {
	case hubmanager.Healthy:
		if protected {
			return assets.Green, "✓ Protected — Claude Code"
		}
		return assets.Yellow, "⚠ Hub running — bridge not configured"
	case hubmanager.Degraded:
		return assets.Yellow, "⚠ Hub degraded"
	case hubmanager.Starting:
		return assets.Grey, "Starting…"
	default:
		return assets.Red, "✕ Hub not running"
	}
}

func (a *app) openSettings() {
	url, err := a.server.EnsureRunning()
	if err != nil {
		log.Printf("settings server: %v", err)
		return
	}
	if err := sysopen.Open(url); err != nil {
		log.Printf("open settings: %v", err)
	}
}

// applySettings reacts to a saved settings change (settings server callback).
func (a *app) applySettings(s settings.Settings) error {
	a.mu.Lock()
	old := a.cur
	a.cur = s
	a.mu.Unlock()

	if s.StartOnLogin != old.StartOnLogin {
		a.applyStartupSetting(s.StartOnLogin)
	}
	if s.HubURL != old.HubURL || s.Mode != old.Mode {
		a.reconfigureHub(s)
	}
	return nil
}

func (a *app) applyStartupSetting(enable bool) {
	if a.startupMgr == nil {
		return
	}
	var err error
	if enable {
		err = a.startupMgr.Enable()
	} else {
		err = a.startupMgr.Disable()
	}
	if err != nil {
		log.Printf("startup registration: %v", err)
	}
}

// reconfigureHub rebuilds the Hub manager when the Hub URL or mode changes.
func (a *app) reconfigureHub(s settings.Settings) {
	if a.hub != nil {
		a.hub.Close()
	}
	a.hub = a.buildHub(s)
	a.hub.SetOnState(a.onHubState)
	a.hub.Run(a.ctx)
	a.hub.Start()
}

func (a *app) openHubConfig() error {
	p, err := config.HubConfigPath()
	if err != nil {
		return err
	}
	return sysopen.Open(p)
}

func (a *app) status() settings.Status {
	a.mu.Lock()
	state, protected := a.hubState, a.protected
	a.mu.Unlock()
	return settings.Status{
		HubState:  state.String(),
		Protected: protected,
		Version:   version.Version,
	}
}

func (a *app) onUpdateAvailable(u updater.Update) {
	a.mu.Lock()
	a.pending = &u
	a.mu.Unlock()
	if a.mUpdate != nil {
		a.mUpdate.SetTitle("Update available — " + u.Version)
	}
	log.Printf("update available: %s", u.Version)
}

func (a *app) onUpdateClicked() {
	a.mu.Lock()
	pending := a.pending
	a.mu.Unlock()
	if pending != nil {
		go a.applyUpdate(*pending)
		return
	}
	go func() {
		u, err := a.updClient.CheckApp(a.ctx, version.Version)
		if err != nil {
			log.Printf("check for updates: %v", err)
			return
		}
		if u.Available {
			a.onUpdateAvailable(u)
		} else if a.mUpdate != nil {
			a.mUpdate.SetTitle("Up to date")
		}
	}()
}

func (a *app) applyUpdate(u updater.Update) {
	log.Printf("applying update %s", u.Version)
	if err := a.updClient.SelfUpdate(a.ctx, u.Release, config.AppAssetName()); err != nil {
		log.Printf("self-update failed: %v", err)
		if a.mUpdate != nil {
			a.mUpdate.SetTitle("Update failed — click to retry")
		}
		return
	}
	if a.hub != nil {
		a.hub.Stop()
	}
	if err := updater.Relaunch(); err != nil {
		log.Printf("relaunch after update: %v", err)
	}
	systray.Quit()
}
