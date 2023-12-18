//go:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package file_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/cli/file"
	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"
)

func TestRecursiveWatcher(t *testing.T) {
	var (
		watcher        *file.RecursiveWatcher
		tmp            string
		eventsExpected []fsnotify.Event
		eventsActual   []fsnotify.Event
	)

	tmp = os.TempDir()
	eventsActual = []fsnotify.Event{}
	eventsExpected = []fsnotify.Event{
		{
			Name: filepath.ToSlash(filepath.Join(tmp, "watch/subdir/testfile")),
			Op:   fsnotify.Create,
		},
		{
			Name: filepath.ToSlash(filepath.Join(tmp, "watch/subdir/testfile")),
			Op:   fsnotify.Chmod,
		},
		{
			Name: filepath.ToSlash(filepath.Join(tmp, "watch/subdir/testfile")),
			Op:   fsnotify.Write,
		},
		{
			Name: filepath.ToSlash(filepath.Join(tmp, "watch/subdir")),
			Op:   fsnotify.Rename,
		},
		{
			Name: filepath.ToSlash(filepath.Join(tmp, "watch/subdir2")),
			Op:   fsnotify.Create,
		},
		{
			Name: filepath.ToSlash(filepath.Join(tmp, "watch/subdir2/testfile")),
			Op:   fsnotify.Rename,
		},
		{
			Name: filepath.ToSlash(filepath.Join(tmp, "watch/subdir2/testfile2")),
			Op:   fsnotify.Create,
		},
		{
			Name: filepath.ToSlash(filepath.Join(tmp, "watch/subdir2/testfile2")),
			Op:   fsnotify.Remove,
		},
	}

	t.Run("Setup Watcher", func(t *testing.T) {
		err := os.MkdirAll(filepath.ToSlash(filepath.Join(tmp, "watch/subdir")), 0755)
		require.NoError(t, err)

		watcher, err = file.NewRecursiveWatcher(uint(len(eventsExpected)))
		require.NoError(t, err)
	})

	t.Run("Watch", func(t *testing.T) {
		// SETUP
		err := watcher.Add(filepath.ToSlash(filepath.Join(tmp, "watch")))
		require.NoError(t, err)

		eventsCh := watcher.Events()
		errorsCh := watcher.Errors()

		go func() {
			for {
				select {
				case e := <-eventsCh:
					if e.Op != 0 {
						eventsActual = append(eventsActual, e)
					}
				case <-errorsCh:
					return
				}
			}
		}()

		// WATCH
		file, err := os.Create(filepath.Join(tmp, "watch/subdir/testfile"))
		require.NoError(t, err)

		err = os.Chmod(filepath.Join(tmp, "watch/subdir/testfile"), 0755)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(tmp, "watch/subdir/testfile"), []byte("write to file"), fs.ModeAppend)
		require.NoError(t, err)

		err = file.Close()
		require.NoError(t, err)

		err = os.Rename(filepath.Join(tmp, "watch/subdir"), filepath.Join(tmp, "watch/subdir2"))
		require.NoError(t, err)

		err = os.Rename(filepath.Join(tmp, "watch/subdir2/testfile"), filepath.Join(tmp, "watch/subdir2/testfile2"))
		require.NoError(t, err)

		err = os.Remove(filepath.Join(tmp, "watch/subdir2/testfile2"))
		require.NoError(t, err)

		// Wait for events to propagate
		time.Sleep(time.Second)

		// CLOSE
		err = watcher.Close()
		require.NoError(t, err)
		require.Empty(t, errorsCh)

		for _, expectedEvent := range eventsExpected {
			require.Contains(t, eventsActual, expectedEvent)
		}
	})

	t.Run("Clean", func(t *testing.T) {
		err := os.RemoveAll(filepath.Join(tmp, "watch"))
		require.NoError(t, err)
	})
}
