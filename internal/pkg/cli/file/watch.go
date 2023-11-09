// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package file

import (
	"io/fs"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// RecursiveWatcher wraps an fsnotify Watcher to recursively watch all files in a directory.
type RecursiveWatcher struct {
	fsnotifyWatcher *fsnotify.Watcher
	done            chan struct{}
	closed          bool
	events          chan fsnotify.Event
	errors          chan error
}

// NewRecursiveWatcher returns a RecursiveWatcher which notifies when changes are made to files inside a recursive directory tree.
func NewRecursiveWatcher(buffer uint) (*RecursiveWatcher, error) {
	watcher, err := fsnotify.NewBufferedWatcher(buffer)
	if err != nil {
		return nil, err
	}

	rw := &RecursiveWatcher{
		events:          make(chan fsnotify.Event, buffer),
		errors:          make(chan error),
		fsnotifyWatcher: watcher,
		done:            make(chan struct{}),
		closed:          false,
	}

	go rw.start()

	return rw, nil
}

// Add recursively adds a directory tree to the list of watched files.
func (rw *RecursiveWatcher) Add(path string) error {
	if rw.closed {
		return fsnotify.ErrClosed
	}
	return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// swallow error from WalkDir, don't attempt to add to watcher.
			return nil
		}
		if d.IsDir() {
			return rw.fsnotifyWatcher.Add(p)
		}
		return nil
	})
}

// Events returns the events channel.
func (rw *RecursiveWatcher) Events() <-chan fsnotify.Event {
	return rw.events
}

// Errors returns the errors channel.
func (rw *RecursiveWatcher) Errors() <-chan error {
	return rw.errors
}

// Close closes the RecursiveWatcher.
func (rw *RecursiveWatcher) Close() error {
	if rw.closed {
		return nil
	}
	rw.closed = true
	close(rw.done)
	return rw.fsnotifyWatcher.Close()
}

func (rw *RecursiveWatcher) start() {
	for {
		select {
		case <-rw.done:
			close(rw.events)
			close(rw.errors)
			return
		case event := <-rw.fsnotifyWatcher.Events:
			// handle recursive watch
			switch event.Op {
			case fsnotify.Create:
				if err := rw.Add(event.Name); err != nil {
					rw.errors <- err
				}
			}

			rw.events <- event
		case err := <-rw.fsnotifyWatcher.Errors:
			rw.errors <- err
		}
	}
}
