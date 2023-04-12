// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package syncbuffer

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncBuffer_Write(t *testing.T) {
	testCases := map[string]struct {
		input        []byte
		wantedOutput string
	}{
		"append to custom buffer with simple input": {
			input:        []byte("hello world"),
			wantedOutput: "hello world",
		},
		"append to custom buffer with empty input": {
			input:        []byte(""),
			wantedOutput: "",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			sb := New()

			// WHEN
			sb.Write(tc.input)

			// THEN
			require.Equal(t, tc.wantedOutput, sb.buf.String())
		})
	}
}

func TestSyncBuffer_IsDone(t *testing.T) {
	testCases := map[string]struct {
		buffer     *SyncBuffer
		wantedDone bool
	}{
		"Buffer is done": {
			buffer:     New(),
			wantedDone: true,
		},
		"Buffer is not done": {
			buffer: New(),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			if tc.wantedDone {
				tc.buffer.MarkDone()
			}

			// WHEN
			actual := tc.buffer.IsDone()

			// THEN
			require.Equal(t, tc.wantedDone, actual)

		})
	}
}

type mockReader struct {
	data []byte
	err  error
}

func (r *mockReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, io.EOF
}

func TestCopy(t *testing.T) {
	testCases := map[string]struct {
		reader       io.Reader
		wantedOutput string
		wantedError  error
	}{
		"copy data from file reader to buffer": {
			reader: &mockReader{
				data: []byte("Building your container image"),
			},
			wantedOutput: "Building your container image",
		},
		"return an error when failed to copy data to buffer": {
			reader:      &mockReader{err: fmt.Errorf("some error")},
			wantedError: fmt.Errorf("some error"),
		},
		"return an EOF error": {
			reader: &mockReader{
				data: []byte("Building your container image"),
				err:  errors.New("EOF"),
			},
			wantedOutput: "Building your container image",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			buf := New()
			gotErr := buf.Copy(tc.reader)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, gotErr.Error())
			}
		})
	}
}

func TestSyncBuffer_WithLabel(t *testing.T) {
	mockSyncBuf := New()
	testCases := map[string]struct {
		label  string
		wanted *LabeledSyncBuffer
	}{
		"if the label is provided": {
			label:  "title",
			wanted: mockSyncBuf.WithLabel("title"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			// THEN
			labeledSyncBuf := mockSyncBuf.WithLabel(tc.label)

			// THEN
			require.Equal(t, tc.wanted, labeledSyncBuf)
		})
	}
}
