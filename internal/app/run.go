// Package app composes the business services, native adapters and native UI and
// owns process lifecycle. It contains no view rendering or on-disk file format.
package app

import (
	"context"
	"copiggy/internal/app/nativeui"
	"copiggy/internal/clipboard"
	"copiggy/internal/clipmodel"
	"copiggy/internal/macos/guard"
	"copiggy/internal/macos/pasteboard"
	"copiggy/internal/settings"
	"copiggy/internal/storage"
	"copiggy/resources"
	"errors"
	"fmt"
	"github.com/egoist/mygo"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

type runtime struct {
	commands  *dispatcher
	history   *clipboard.History
	platform  nativeClipboard
	view      *nativeui.Controller
	prefs     *storage.PreferencesRepository
	repo      *storage.HistoryRepository
	observing atomic.Bool
	stopping  atomic.Bool
	quit      chan struct{}
	// These fields belong exclusively to the serial command worker.
	monitoring                bool
	lastRevision, ownRevision int64
	lastOrdinary, lastExpiry  time.Time
	lastHistoryError          string
}

func Run() error {
	ctx := context.Background()
	repo := storage.NewHistoryRepository("", "")
	prefsRepo := &storage.PreferencesRepository{Writer: repo.Writer}
	prefs, preferenceErr := prefsRepo.Load(ctx)
	if preferenceErr != nil {
		log.Printf("load preferences: %v", preferenceErr)
		prefs = settings.Default()
	}
	history := clipboard.NewHistory(repo, settings.Retention(prefs.RetentionDays))
	historyErr := history.Load(ctx)
	var startupErr error
	mygo.App.SetName("Copiggy")
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.WhenReady(func() {
		// A bundled app uses its compiled Icon Composer resource. The PNG fallback
		// is only for running the development executable without a bundle.
		executable, _ := os.Executable()
		if !strings.Contains(filepath.ToSlash(executable), ".app/Contents/MacOS/") {
			if err := mygo.App.Dock.SetIcon(resources.AppIcon); err != nil {
				log.Printf("set application icon: %v", err)
			}
		}
		reader, err := pasteboard.New()
		if err != nil {
			startupErr = fmt.Errorf("initialize pasteboard: %w", err)
			mygo.App.Quit()
			return
		}
		bridge, err := guard.New()
		if err != nil {
			startupErr = fmt.Errorf("initialize native guard: %w", err)
			mygo.App.Quit()
			return
		}
		r := &runtime{commands: newDispatcher(), history: history, platform: nativeClipboard{reader, bridge}, prefs: prefsRepo, repo: repo, monitoring: settings.Enabled(prefs.RecordingEnabled), lastRevision: reader.ChangeCount(), ownRevision: -1, quit: make(chan struct{}), lastExpiry: time.Now()}
		r.view = nativeui.New(nativeui.Config{
			History: history, Guard: bridge, Preferences: prefs, Dispatch: r.dispatch, Refresh: r.observeSoon,
			SavePreferences: func(p settings.Preferences) error {
				if err := prefsRepo.Save(context.Background(), p); err != nil {
					return err
				}
				history.SetRetention(settings.Retention(p.RetentionDays))
				return nil
			},
			SetMonitoring: func(enabled bool) {
				r.commands.submit(func() { r.monitoring = enabled; r.lastRevision = r.platform.ChangeCount() })
			},
			SetGuard: func(enabled bool) error {
				if !enabled {
					bridge.StopGuard()
					return nil
				}
				bridge.RequestAccessibility()
				return bridge.StartGuard()
			},
			Copy: r.copy, CopyVariant: r.copyVariant,
		})
		if historyErr != nil {
			fallback := "Could not load history"
			if errors.Is(historyErr, clipboard.ErrHistorySave) {
				fallback = "Could not save history"
			}
			r.view.Report("load clipboard history", historyErr, fallback)
		}
		if settings.Enabled(prefs.PasteGuardEnabled) {
			if err := bridge.StartGuard(); err != nil {
				r.view.Report("start paste guard", err, "Could not start Paste Guard")
			}
		}
		canQuit := false
		mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
			if canQuit {
				return
			}
			e.PreventDefault()
			if r.stopping.Swap(true) {
				return
			}
			r.view.BeginQuit()
			r.commands.seal(func() {
				err := history.Persist(context.Background())
				repo.Close()
				mygo.RunOnMain(func() {
					if err != nil {
						log.Printf("save history on quit: %v", err)
					}
					canQuit = true
					mygo.App.Quit()
				})
			})
		})
		mygo.App.OnWillQuit(func(*mygo.QuitEvent) { close(r.quit); bridge.StopGuard(); r.view.Close() })
		r.view.Show()
		r.watch()
	})
	err := mygo.App.Run()
	repo.Close()
	if err != nil {
		return err
	}
	return startupErr
}
func (r *runtime) dispatch(work func() error, done func(error)) {
	r.commands.submit(func() { err := work(); mygo.RunOnMain(func() { done(err) }) })
}
func (r *runtime) copy(ctx context.Context, id string, draft bool) (clipmodel.Item, error) {
	service := clipboard.Copy{History: r.history, Port: r.platform}
	var item clipmodel.Item
	var revision int64
	var err error
	if draft {
		item, revision, err = service.Draft(ctx, id)
	} else {
		item, revision, err = service.Item(ctx, id)
	}
	if err == nil {
		r.ownRevision, r.lastRevision = revision, revision
	}
	return item, err
}

func (r *runtime) copyVariant(ctx context.Context, id string, format clipmodel.CopyFormat) (clipmodel.Item, error) {
	service := clipboard.Copy{History: r.history, Port: r.platform}
	item, revision, err := service.Variant(ctx, id, format)
	if err == nil {
		// Keep derived writes out of capture just like a restored history item.
		r.ownRevision, r.lastRevision = revision, revision
	}
	return item, err
}
func (r *runtime) observeSoon() {
	if r.stopping.Load() || r.observing.Swap(true) {
		return
	}
	r.commands.submit(func() { defer r.observing.Store(false); r.observe() })
}
func (r *runtime) watch() {
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if r.stopping.Load() {
					continue
				}
				mygo.RunOnMain(func() {
					if r.stopping.Load() {
						return
					}
					r.view.Tick()
					for {
						select {
						case event := <-r.platform.guard.Events():
							r.view.Event(event)
						default:
							r.observeSoon()
							return
						}
					}
				})
			case <-r.quit:
				return
			}
		}
	}()
}
func (r *runtime) changed() {
	mygo.RunOnMain(func() {
		if !r.stopping.Load() {
			r.view.Changed()
		}
	})
}
func (r *runtime) reportHistoryError(err error) {
	if err == nil {
		r.lastHistoryError = ""
		return
	}
	if err.Error() == r.lastHistoryError {
		return
	}
	r.lastHistoryError = err.Error()
	mygo.RunOnMain(func() {
		if !r.stopping.Load() {
			r.view.Report("save history", err, "Could not save history")
		}
	})
}
func (r *runtime) observe() {
	if r.stopping.Load() {
		return
	}
	if time.Since(r.lastExpiry) >= time.Minute {
		changed, err := r.history.Expire(context.Background(), time.Now())
		if err == nil {
			err = r.history.Persist(context.Background())
		}
		r.reportHistoryError(err)
		if changed {
			r.changed()
		}
		r.lastExpiry = time.Now()
	}
	if _, pending := r.platform.PendingCopy(); pending {
		captured, ok := clipboard.CompleteProtectedCopy(context.Background(), r.platform, r.platform.read, r.history)
		if ok {
			r.lastRevision = captured.Revision
			r.reportHistoryError(captured.SaveError)
			r.changed()
		}
		return
	}
	if time.Since(r.lastOrdinary) < 300*time.Millisecond {
		return
	}
	r.lastOrdinary = time.Now()
	revision := r.platform.ChangeCount()
	if revision == r.lastRevision {
		return
	}
	if revision == r.ownRevision {
		r.lastRevision = revision
		return
	}
	if !r.monitoring {
		r.lastRevision = revision
		r.history.ClearCurrent()
		r.changed()
		return
	}
	source := r.platform.Frontmost()
	data, err := r.platform.read()
	if _, pending := r.platform.PendingCopy(); pending {
		return
	}
	if err != nil || revision != r.platform.ChangeCount() {
		return
	}
	// Validate off the AppKit thread, then recheck both revision and pending
	// native intent before publishing this ordinary observation.
	_, err = clipmodel.ContentFromRepresentations(data)
	if _, pending := r.platform.PendingCopy(); pending {
		return
	}
	if revision != r.platform.ChangeCount() {
		return
	}
	if err != nil {
		r.lastRevision = revision
		r.history.ClearCurrent()
		r.changed()
		mygo.RunOnMain(func() {
			if !r.stopping.Load() {
				r.view.Report("read clipboard content", err, "Clipboard content unavailable")
			}
		})
		return
	}
	if after := r.platform.Frontmost(); after.PID != source.PID {
		source = clipboard.SourceApp{}
	}
	_, _, err = r.history.Capture(context.Background(), data, clipmodel.Source{Name: source.Name, BundleID: source.BundleID}, nil)
	r.lastRevision = revision
	if r.platform.ChangeCount() != revision {
		r.history.ClearCurrent()
	}
	if err != nil {
		r.reportHistoryError(err)
	}
	r.changed()
}
