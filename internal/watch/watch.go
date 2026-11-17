// Package watch reports external changes inside a workspace so the editor
// can reload clean files, prompt for dirty ones and refresh the explorer
// incrementally.
package watch

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// EventName is the frontend event channel for workspace changes.
const EventName = "workspace-changed"

// Change is one coalesced filesystem change, relative to the workspace root.
type Change struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"` // create | write | remove
	Directory bool   `json:"directory"`
	Size      int64  `json:"size"`
}

// Batch is delivered after changes settle. Overflow asks the frontend to do
// a full refresh because too many paths changed to list individually.
type Batch struct {
	Root     string   `json:"root"`
	Changes  []Change `json:"changes"`
	Overflow bool     `json:"overflow"`
}

const (
	settleDelay     = 200 * time.Millisecond
	maxBatchChanges = 500
	maxWatchedDirs  = 4000
)

// Ignored reports directories that are never watched or listed: hidden
// folders, build output and dependency/vendor trees.
func Ignored(name string) bool {
	switch name {
	case "build", "node_modules", "vendor":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// Watcher observes one workspace root.
type Watcher struct {
	root    string
	notify  *fsnotify.Watcher
	sink    func(Batch)
	done    chan struct{}
	stopped chan struct{}
	once    sync.Once
	dirs    int
}

// Start begins watching root. The sink is called from a single goroutine.
func Start(root string, sink func(Batch)) (*Watcher, error) {
	notify, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{root: root, notify: notify, sink: sink, done: make(chan struct{}), stopped: make(chan struct{})}
	w.addTree(root)
	go w.loop()
	return w, nil
}

// addTree watches a directory and its visible descendants. Links are never
// followed, so a watch cannot leave the workspace.
func (w *Watcher) addTree(dir string) {
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		if path != w.root && Ignored(entry.Name()) {
			return filepath.SkipDir
		}
		if w.dirs >= maxWatchedDirs {
			return filepath.SkipAll
		}
		if w.notify.Add(path) == nil {
			w.dirs++
		}
		return nil
	})
}

// relative converts an event path to a slash-separated workspace path and
// rejects anything inside an ignored directory.
func (w *Watcher) relative(path string) (string, bool) {
	rel, err := filepath.Rel(w.root, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	for _, part := range strings.Split(rel, "/") {
		if Ignored(part) || strings.HasSuffix(part, ".tmp") {
			return "", false
		}
	}
	return rel, true
}

func (w *Watcher) loop() {
	defer close(w.stopped)
	pending := map[string]Change{}
	overflow := false
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	flush := func() {
		if len(pending) == 0 && !overflow {
			return
		}
		batch := Batch{Root: w.root, Changes: make([]Change, 0, len(pending)), Overflow: overflow}
		for _, change := range pending {
			batch.Changes = append(batch.Changes, change)
		}
		sort.Slice(batch.Changes, func(i, j int) bool { return batch.Changes[i].Path < batch.Changes[j].Path })
		pending, overflow = map[string]Change{}, false
		w.sink(batch)
	}
	for {
		select {
		case <-w.done:
			return
		case <-timer.C:
			flush()
		case _, ok := <-w.notify.Errors:
			if !ok {
				return
			}
			// A dropped kernel queue means changes were missed.
			overflow = true
			timer.Reset(settleDelay)
		case event, ok := <-w.notify.Events:
			if !ok {
				return
			}
			rel, visible := w.relative(event.Name)
			if !visible {
				continue
			}
			change := Change{Path: rel}
			info, statErr := os.Lstat(event.Name)
			switch {
			case statErr != nil:
				change.Kind = "remove"
				if previous, seen := pending[rel]; seen {
					change.Directory = previous.Directory
				}
			case info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0:
				continue
			default:
				change.Directory, change.Size = info.IsDir(), info.Size()
				if event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
					change.Kind = "create"
					if info.IsDir() {
						w.addTree(event.Name)
					}
				} else {
					change.Kind = "write"
					if info.IsDir() {
						continue
					}
				}
				// A file created and then written is still a creation.
				if previous, seen := pending[rel]; seen && previous.Kind == "create" {
					change.Kind = "create"
				}
			}
			if len(pending) >= maxBatchChanges {
				overflow = true
			} else {
				pending[rel] = change
			}
			timer.Reset(settleDelay)
		}
	}
}

// Stop ends the watch and waits for the delivery goroutine to finish.
func (w *Watcher) Stop() {
	w.once.Do(func() {
		close(w.done)
		_ = w.notify.Close()
		<-w.stopped
	})
}
