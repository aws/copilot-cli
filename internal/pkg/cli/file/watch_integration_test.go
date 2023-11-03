// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package file_test

import (
	"fmt"
	"io/fs"
	"os"
	"testing"

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

		watcher, err = file.NewRecursiveWatcher()
		require.NoError(t, err)
	})

	t.Run("Watch", func(t *testing.T) {
		// SETUP
		err := watcher.Add(fmt.Sprintf("%s/watch", tmp))
		require.NoError(t, err)

		eventsCh := watcher.Events()
		errorsCh := watcher.Errors()

		// WATCH
		file, err := os.Create(fmt.Sprintf("%s/watch/subdir/testfile", tmp))
		require.NoError(t, err)
		eventsActual = append(eventsActual, <-eventsCh)

		err = os.Chmod(fmt.Sprintf("%s/watch/subdir/testfile", tmp), 0755)
		require.NoError(t, err)
		eventsActual = append(eventsActual, <-eventsCh)

		err = os.WriteFile(fmt.Sprintf("%s/watch/subdir/testfile", tmp), []byte("write to file"), fs.ModeAppend)
		require.NoError(t, err)
		eventsActual = append(eventsActual, <-eventsCh)

		err = file.Close()
		require.NoError(t, err)

		err = os.Rename(fmt.Sprintf("%s/watch/subdir", tmp), fmt.Sprintf("%s/watch/subdir2", tmp))
		require.NoError(t, err)
		eventsActual = append(eventsActual, <-eventsCh)
		eventsActual = append(eventsActual, <-eventsCh)
		eventsActual = append(eventsActual, <-eventsCh)

		err = os.Rename(fmt.Sprintf("%s/watch/subdir2/testfile", tmp), fmt.Sprintf("%s/watch/subdir2/testfile2", tmp))
		require.NoError(t, err)
		eventsActual = append(eventsActual, <-eventsCh)
		eventsActual = append(eventsActual, <-eventsCh)

		err = os.Remove(fmt.Sprintf("%s/watch/subdir2/testfile2", tmp))
		require.NoError(t, err)
		eventsActual = append(eventsActual, <-eventsCh)

		// CLOSE
		err = watcher.Close()
		require.NoError(t, err)
		require.Empty(t, errorsCh)

		require.Equal(t, eventsExpected, eventsActual)
	})

	t.Run("Clean", func(t *testing.T) {
		err := os.RemoveAll(fmt.Sprintf("%s/watch", tmp))
		require.NoError(t, err)
	})
}
