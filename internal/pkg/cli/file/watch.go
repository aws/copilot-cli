// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package file

import (
	"io/fs"
	"os"
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
func NewRecursiveWatcher(dir string) (*RecursiveWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	rw := &RecursiveWatcher{
		events:          make(chan fsnotify.Event),
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
	if err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return rw.fsnotifyWatcher.Add(path)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// Remove recursively removes a directory tree from the list of watched files.
func (rw *RecursiveWatcher) Remove(path string) error {
	if rw.closed {
		return nil
	}
	if err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return rw.fsnotifyWatcher.Remove(path)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
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
	rw.closed = true
	close(rw.done)
	return rw.fsnotifyWatcher.Close()
}

func (rw *RecursiveWatcher) start() {
	for {
		select {
		case event := <-rw.fsnotifyWatcher.Events:
			info, err := os.Stat(event.Name)
			if err != nil {
				break
			}
			if info.IsDir() {
				switch event.Op {
				case fsnotify.Create:
					err := rw.Add(event.Name)
					if err != nil {
						rw.errors <- err
					}
				case fsnotify.Remove:
					err := rw.Remove(event.Name)
					if err != nil {
						rw.errors <- err
					}
				}
			} else {
				rw.events <- event
			}
		case err := <-rw.fsnotifyWatcher.Errors:
			rw.errors <- err
		case <-rw.done:
			close(rw.events)
			close(rw.errors)
			return
		}
	}
}
