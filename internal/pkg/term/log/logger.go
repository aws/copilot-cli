// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"io"
)

// Logger represents a logging object that writes to an io.Writer lines of outputs.
type Logger struct {
	w io.Writer
}

// New creates a new Logger.
func New(w io.Writer) *Logger {
	return &Logger{
		w: w,
	}
}

// Success writes args prefixed with a "✔ Success!".
func (l *Logger) Success(args ...interface{}) {
	success(l.w, args...)
}

// Successln writes args prefixed with a "✔ Success!" and a new line.
func (l *Logger) Successln(args ...interface{}) {
	successln(l.w, args...)
}

// Successf formats according to the specifier, prefixes the message with a "✔ Success!", and writes it.
func (l *Logger) Successf(format string, args ...interface{}) {
	successf(l.w, format, args...)
}

// Error writes args prefixed with "✘ Error!".
func (l *Logger) Error(args ...interface{}) {
	err(l.w, args...)
}

// Errorln writes args prefixed with a "✘ Error!" and a new line.
func (l *Logger) Errorln(args ...interface{}) {
	errln(l.w, args...)
}

// Errorf formats according to the specifier, prefixes the message with a "✘ Error!", and writes it.
func (l *Logger) Errorf(format string, args ...interface{}) {
	errf(l.w, format, args...)
}

// Warning  writes args prefixed with "Note:".
func (l *Logger) Warning(args ...interface{}) {
	warning(l.w, args...)
}

// Warningln writes args prefixed with a "Note:" and a new line.
func (l *Logger) Warningln(args ...interface{}) {
	warningln(l.w, args...)
}

// Warningf formats according to the specifier, prefixes the message with a "Note:", and writes it.
func (l *Logger) Warningf(format string, args ...interface{}) {
	warningf(l.w, format, args...)
}

// Info writes the message.
func (l *Logger) Info(args ...interface{}) {
	info(l.w, args...)
}

// Infoln writes the message with a new line.
func (l *Logger) Infoln(args ...interface{}) {
	infoln(l.w, args...)
}

// Infof formats according to the specifier, and writes the message.
func (l *Logger) Infof(format string, args ...interface{}) {
	infof(l.w, format, args...)
}

// Debug writes the message.
func (l *Logger) Debug(args ...interface{}) {
	debug(l.w, args...)
}

// Debugln writes the message and with a new line.
func (l *Logger) Debugln(args ...interface{}) {
	debugln(l.w, args...)
}

// Debugf formats according to the specifier, and writes the message.
func (l *Logger) Debugf(format string, args ...interface{}) {
	debugf(l.w, format, args...)
}
