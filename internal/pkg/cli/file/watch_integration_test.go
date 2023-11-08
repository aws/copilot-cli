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
		eventsExpected map[fsnotify.Event]struct{}
		eventsActual   map[fsnotify.Event]struct{}
	)

	tmp = os.TempDir()
	eventsActual = make(map[fsnotify.Event]struct{})
	eventsExpected = map[fsnotify.Event]struct{}{
		{
			Name: fmt.Sprintf("%s/watch/subdir/testfile", tmp),
			Op:   fsnotify.Create,
		}: {},
		{
			Name: fmt.Sprintf("%s/watch/subdir/testfile", tmp),
			Op:   fsnotify.Chmod,
		}: {},
		{
			Name: fmt.Sprintf("%s/watch/subdir/testfile", tmp),
			Op:   fsnotify.Write,
		}: {},
		{
			Name: fmt.Sprintf("%s/watch/subdir", tmp),
			Op:   fsnotify.Rename,
		}: {},
		{
			Name: fmt.Sprintf("%s/watch/subdir2", tmp),
			Op:   fsnotify.Create,
		}: {},
		{
			Name: fmt.Sprintf("%s/watch/subdir2/testfile", tmp),
			Op:   fsnotify.Rename,
		}: {},
		{
			Name: fmt.Sprintf("%s/watch/subdir2/testfile2", tmp),
			Op:   fsnotify.Create,
		}: {},
		{
			Name: fmt.Sprintf("%s/watch/subdir2/testfile2", tmp),
			Op:   fsnotify.Remove,
		}: {},
	}

	t.Run("Setup Watcher", func(t *testing.T) {
		err := os.MkdirAll(fmt.Sprintf("%s/watch/subdir", tmp), 0755)
		require.NoError(t, err)

		watcher, err = file.NewRecursiveWatcher(uint(len(eventsExpected)))
		require.NoError(t, err)
	})

	t.Run("Watch", func(t *testing.T) {
		// SETUP
		err := watcher.Add(fmt.Sprintf("%s/watch", tmp))
		require.NoError(t, err)

		eventsCh := watcher.Events()
		errorsCh := watcher.Errors()

		expectAndPopulateEvents := func(t *testing.T, n int, events map[fsnotify.Event]struct{}) {
			for i := 0; i < n; i++ {
				select {
				case e := <-eventsCh:
					events[e] = struct{}{}
				case <-time.After(time.Second):
				}
			}
		}

		// WATCH
		file, err := os.Create(fmt.Sprintf("%s/watch/subdir/testfile", tmp))
		require.NoError(t, err)
		expectAndPopulateEvents(t, 1, eventsActual)

		err = os.Chmod(fmt.Sprintf("%s/watch/subdir/testfile", tmp), 0755)
		require.NoError(t, err)
		expectAndPopulateEvents(t, 1, eventsActual)

		err = os.WriteFile(fmt.Sprintf("%s/watch/subdir/testfile", tmp), []byte("write to file"), fs.ModeAppend)
		require.NoError(t, err)
		expectAndPopulateEvents(t, 2, eventsActual)

		err = file.Close()
		require.NoError(t, err)

		err = os.Rename(fmt.Sprintf("%s/watch/subdir", tmp), fmt.Sprintf("%s/watch/subdir2", tmp))
		require.NoError(t, err)
		expectAndPopulateEvents(t, 3, eventsActual)

		err = os.Rename(fmt.Sprintf("%s/watch/subdir2/testfile", tmp), fmt.Sprintf("%s/watch/subdir2/testfile2", tmp))
		require.NoError(t, err)
		expectAndPopulateEvents(t, 2, eventsActual)

		err = os.Remove(fmt.Sprintf("%s/watch/subdir2/testfile2", tmp))
		require.NoError(t, err)
		expectAndPopulateEvents(t, 1, eventsActual)

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
