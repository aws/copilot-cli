// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockerfile

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// instructionName identifies the name of the instruction.
type instructionName int

const (
	instrErr         instructionName = iota // an error occurred while scanning.
	instrHealthCheck                        // a HEALTHCHECK instruction.
	instrExpose                             // an EXPOSE instruction.
	instrEOF                                // done scanning.
)

const (
	markerExposeInstr      = "expose "      // start of an EXPOSE instruction.
	markerHealthCheckInstr = "healthcheck " // start of a HEALTHCHECK instruction.
)

var (
	lineContinuationMarkers = []string{"`", "\\"} // denotes that the instruction continues to the next line.

	instrMarkers = map[instructionName]string{ // lookup table for how an instruction starts.
		instrExpose:      markerExposeInstr,
		instrHealthCheck: markerHealthCheckInstr,
	}
)

// An instruction part of a Dockerfile.
// Dockerfiles are of the following format:
// ```
// # Comment
// INSTRUCTION arguments
// ```
type instruction struct {
	name instructionName // the type of the instruction.
	args string          // the arguments of an instruction.
	line int             // line number at the start of this instruction.
}

// lexer holds the state of the scanner.
type lexer struct {
	scanner *bufio.Scanner // line-by-line scanner of the contents of the Dockerfile.

	curLineCount int              // line number scanned so far.
	curLine      string           // current line scanned.
	curArgs      *strings.Builder // accumulated arguments for an instruction.

	instructions chan instruction //channel of discovered instructions.
}

// lex returns a running lexer that scans the Dockerfile.
// The lexing logic is heavily inspired by:
//   https://cs.opensource.google/go/go/+/refs/tags/go1.17.1:src/text/template/parse/lex.go
func lex(reader io.Reader) *lexer {
	l := &lexer{
		scanner:      bufio.NewScanner(reader),
		curArgs:      new(strings.Builder),
		instructions: make(chan instruction),
	}
	go l.run()
	return l
}

// next returns the next scanned instruction.
func (lex *lexer) next() instruction {
	return <-lex.instructions
}

// readLine loads the next line in the Dockerfile.
// If we reached the end of the file, then isEOF is set to true.
// If any unexpected error occurs during scanning, then err is not nil.
func (lex *lexer) readLine() (isEOF bool, err error) {
	if ok := lex.scanner.Scan(); !ok {
		if err := lex.scanner.Err(); err != nil {
			return false, err
		}
		return true, nil
	}
	lex.curLineCount++
	lex.curLine = lex.scanner.Text()
	return false, nil
}

// emit passes an instruction back to the client.
func (lex *lexer) emit(name instructionName) {
	defer lex.curArgs.Reset()

	lex.instructions <- instruction{
		name: name,
		args: lex.curArgs.String(),
		line: lex.curLineCount,
	}
}

// emitErr notifies clients that an error occurred during scanning.
func (lex *lexer) emitErr(err error) {
	lex.instructions <- instruction{
		name: instrErr,
		args: err.Error(),
		line: lex.curLineCount,
	}
}

// consumeInstr keeps calling readLine and storing the arguments in the lexer until there is no more
// continuation marker and then emits the instruction.
func (lex *lexer) consumeInstr(name instructionName) stateFn {
	isEOF, err := lex.readLine()
	if err != nil {
		lex.emitErr(err)
		return nil
	}
	if isEOF {
		lex.emitErr(fmt.Errorf("unexpected EOF while reading Dockerfile at line %d", lex.curLineCount))
		return nil
	}

	// For example a healthcheck instruction like:
	//  ```
	//  HEALTHCHECK --interval=5m --timeout=3s --start-period=2s\
	//     --retries=3 \
	//	  CMD curl -f http://localhost/ || exit 1`
	//  ```
	//  will be stored as:
	//  curArgs = "--interval=5m --timeout=3s --start-period=2s --retries=3  CMD curl -f http://localhost/ || exit 1"
	clean := trimContinuationLineMarker(trimLeadingWhitespaces(lex.curLine))
	_, err = lex.curArgs.WriteString(fmt.Sprintf(" %s", clean)) // separate each new line with a space character.
	if err != nil {
		lex.emitErr(fmt.Errorf("write '%s' to arguments buffer: %w", clean, err))
		return nil
	}

	if hasLineContinuationMarker(lex.curLine) {
		return lex.consumeInstr(name)
	}
	lex.emit(name)
	return lexContent
}

// run walks through the state machine for the lexer.
func (lex *lexer) run() {
	for state := lexContent; state != nil; {
		state = state(lex)
	}
	close(lex.instructions)
}

// stateFn represents a state machine transition of the scanner going from one INSTRUCTION to the next.
type stateFn func(*lexer) stateFn

// lexContent scans until we reach the end of the Dockerfile.
func lexContent(l *lexer) stateFn {
	isEOF, err := l.readLine()
	if err != nil {
		l.emitErr(err)
		return nil
	}
	if isEOF {
		l.emit(instrEOF)
		return nil
	}
	line := strings.ToLower(strings.TrimLeftFunc(l.curLine, unicode.IsSpace))
	switch {
	case strings.HasPrefix(line, markerExposeInstr):
		return lexExpose
	case strings.HasPrefix(line, markerHealthCheckInstr):
		return lexHealthCheck
	default:
		return lexContent // Ignore all the other instructions, consume the line without emitting any instructions.
	}
}

// lexExpose collects the arguments for an EXPOSE instruction and then emits it.
func lexExpose(l *lexer) stateFn {
	return lexInstruction(l, instrExpose)
}

// lexHealthCheck collects the arguments for a HEALTHCHECK instruction and then emits it.
func lexHealthCheck(l *lexer) stateFn {
	return lexInstruction(l, instrHealthCheck)
}

// lexInstruction collects all the arguments for the named instruction and then emits it.
func lexInstruction(l *lexer, name instructionName) stateFn {
	args := trimContinuationLineMarker(trimInstruction(l.curLine, instrMarkers[name]))
	_, err := l.curArgs.WriteString(args)
	if err != nil {
		l.emitErr(fmt.Errorf("write '%s' to arguments buffer: %w", args, err))
		return nil
	}

	if hasLineContinuationMarker(l.curLine) {
		return l.consumeInstr(name)
	}
	l.emit(name)
	return lexContent
}

// hasLineContinuationMarker returns true if the line wraps to the next line.
func hasLineContinuationMarker(line string) bool {
	for _, marker := range lineContinuationMarkers {
		if strings.HasSuffix(line, marker) {
			return true
		}
	}
	return false
}

// trimInstruction trims the instrMarker prefix from line and returns it.
func trimInstruction(line, instrMarker string) string {
	normalized := strings.ToLower(line)
	if !strings.Contains(normalized, instrMarker) {
		return line
	}
	idx := strings.Index(normalized, instrMarker) + len(instrMarker)
	return line[idx:]
}

// trimContinuationLineMarker returns the line without any continuation line markers.
// If the line doesn't have a continuation marker, then returns it as is.
func trimContinuationLineMarker(line string) string {
	for _, marker := range lineContinuationMarkers {
		if strings.HasSuffix(line, marker) {
			return strings.TrimSuffix(line, marker)
		}
	}
	return line
}

// trimLeadingWhitespaces removes any leading space characters.
func trimLeadingWhitespaces(line string) string {
	return strings.TrimLeftFunc(line, unicode.IsSpace)
}
