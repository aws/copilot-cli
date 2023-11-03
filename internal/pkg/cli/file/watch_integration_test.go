//go:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package file_test

import (
	"fmt"
	"io/fs"
	"os"
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
	eventsActual = make([]fsnotify.Event, 0)
	eventsExpected = []fsnotify.Event{
		{
			Name: fmt.Sprintf("%s/watch/subdir/testfile", tmp),
			Op:   fsnotify.Create,
		},
		{
			Name: fmt.Sprintf("%s/watch/subdir/testfile", tmp),
			Op:   fsnotify.Chmod,
		},
		{
			Name: fmt.Sprintf("%s/watch/subdir/testfile", tmp),
			Op:   fsnotify.Write,
		},
		{
			Name: fmt.Sprintf("%s/watch/subdir", tmp),
			Op:   fsnotify.Rename,
		},
		{
			Name: fmt.Sprintf("%s/watch/subdir2", tmp),
			Op:   fsnotify.Create,
		},
		{
			Name: fmt.Sprintf("%s/watch/subdir", tmp),
			Op:   fsnotify.Rename,
		},
		{
			Name: fmt.Sprintf("%s/watch/subdir2/testfile", tmp),
			Op:   fsnotify.Rename,
		},
		{
			Name: fmt.Sprintf("%s/watch/subdir2/testfile2", tmp),
			Op:   fsnotify.Create,
		},
		{
			Name: fmt.Sprintf("%s/watch/subdir2/testfile2", tmp),
			Op:   fsnotify.Remove,
		},
	}

	t.Run("Setup Watcher", func(t *testing.T) {
		err := os.MkdirAll(fmt.Sprintf("%s/watch/subdir", tmp), 0755)
		require.NoError(t, err)

		watcher, err = file.NewRecursiveWatcher(uint(len(eventsExpected)))
		require.NoError(t, err)
	})

	t.Run("Handle Events", func(t *testing.T) {
		eventsCh := watcher.Events()
		errorsCh := watcher.Errors()
		go func() {
			for {
				select {
				case e, ok := <-eventsCh:
					if !ok {
						require.Empty(t, errorsCh)
						return
					}
					eventsActual = append(eventsActual, e)
				case e, ok := <-errorsCh:
					require.NoError(t, e)
					if !ok {
						require.Empty(t, errorsCh)
						return
					}
				}

			}
		}()
	})

	t.Run("Watch", func(t *testing.T) {
		// SETUP
		err := watcher.Add(fmt.Sprintf("%s/watch", tmp))
		require.NoError(t, err)

		// WATCH
		f, err := os.Create(fmt.Sprintf("%s/watch/subdir/testfile", tmp))
		require.NoError(t, err)

		err = os.Chmod(fmt.Sprintf("%s/watch/subdir/testfile", tmp), 0755)
		require.NoError(t, err)

		err = os.WriteFile(fmt.Sprintf("%s/watch/subdir/testfile", tmp), []byte("write to file"), fs.ModeAppend)
		require.NoError(t, err)

		err = f.Close()
		require.NoError(t, err)

		err = os.Rename(fmt.Sprintf("%s/watch/subdir", tmp), fmt.Sprintf("%s/watch/subdir2", tmp))
		require.NoError(t, err)

		// filepath.WalkDir is slow, wait to prevent race condition
		time.Sleep(100 * time.Millisecond)

		err = os.Rename(fmt.Sprintf("%s/watch/subdir2/testfile", tmp), fmt.Sprintf("%s/watch/subdir2/testfile2", tmp))
		require.NoError(t, err)

		// filepath.WalkDir is slow, wait to prevent race condition
		time.Sleep(100 * time.Millisecond)

		err = os.Remove(fmt.Sprintf("%s/watch/subdir2/testfile2", tmp))
		require.NoError(t, err)

		// CLOSE
		err = watcher.Close()
		require.NoError(t, err)
	})

	t.Run("Clean", func(t *testing.T) {
		err := os.RemoveAll(fmt.Sprintf("%s/watch", tmp))
		require.NoError(t, err)

		require.ElementsMatch(t, eventsExpected, eventsActual)
	})
}
