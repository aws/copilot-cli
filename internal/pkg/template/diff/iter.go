// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"gopkg.in/yaml.v3"
)

type action int

const (
	actionMatch action = iota
	actionMod
	actionDel
	actionInsert
	actionDone
)

type tracker[T any] struct {
	index int
	data  []T
}

func newLCSStateMachine(fromSeq, toSeq []yaml.Node, lcsIndices []lcsIndex) lcsStateMachine {
	return lcsStateMachine{
		from:       tracker[yaml.Node]{data: fromSeq},
		to:         tracker[yaml.Node]{data: toSeq},
		lcsIndices: tracker[lcsIndex]{data: lcsIndices},
	}
}

type lcsStateMachine struct {
	from       tracker[yaml.Node]
	to         tracker[yaml.Node]
	lcsIndices tracker[lcsIndex]
	currAction action
}

func (sm *lcsStateMachine) action() action {
	var action action
	var (
		lcsDone  = sm.lcsIndices.index >= len(sm.lcsIndices.data)
		fromDone = sm.from.index >= len(sm.from.data)
		toDone   = sm.to.index >= len(sm.to.data)
	)
	if lcsDone {
		switch {
		case fromDone && toDone:
			action = actionDone
		case toDone:
			// Ex: "a,d,e" -> "a". When the lcsStateMachine moves to "d" in "from", both lcs and "to" are done, and "d,e" are considered deleted.
			action = actionDel
		case fromDone:
			// Ex: "a" -> "a,d,e". When the lcsStateMachine moves to "d" in "to", both lcs and "from" are done, and "d,e" are considered to be inserted.
			action = actionInsert
		default:
			// Ex: "a,b" -> "a,c". When the lcsStateMachine moves to (b,c), lcs is done, and b is modified into c.
			action = actionMod
		}
		sm.currAction = action
		return action
	}
	commonIdx := sm.lcsIndices.data[sm.lcsIndices.index]
	switch {
	case sm.from.index == commonIdx.inA && sm.to.index == commonIdx.inB:
		action = actionMatch
	case sm.from.index != commonIdx.inA && sm.to.index != commonIdx.inB:
		action = actionMod
	case sm.from.index != commonIdx.inA:
		// Ex: "a,b,c" -> "c,1,2". When the lcsStateMachine is at (a,c /(b,c), only "c" is common, a,b are considered deleted.
		action = actionDel
	default:
		// Ex: "a,b,c" -> "1,2,a". When the lcsStateMachine is at (a,1) and (a,2), only "a" is common, "1,2" are considered inserted.
		action = actionInsert
	}
	sm.currAction = action
	return action
}

func (sm *lcsStateMachine) peek() action {
	lcsIdxOld, fromIdxOld, toIdxOld, actionOld := sm.lcsIndices.index, sm.from.index, sm.to.index, sm.currAction
	sm.next()
	next := sm.action()
	sm.lcsIndices.index, sm.from.index, sm.to.index, sm.currAction = lcsIdxOld, fromIdxOld, toIdxOld, actionOld
	return next
}

func (sm *lcsStateMachine) next() {
	switch sm.currAction {
	case actionMatch:
		sm.lcsIndices.index++
		fallthrough
	case actionMod:
		sm.from.index++
		sm.to.index++
	case actionDel:
		sm.from.index++
	case actionInsert:
		sm.to.index++
	}
}

func (sm *lcsStateMachine) fromItem() yaml.Node {
	return sm.from.data[sm.from.index]
}

func (sm *lcsStateMachine) toItem() yaml.Node {
	return sm.to.data[sm.to.index]
}

func (sm *lcsStateMachine) fromIndex() int {
	return sm.from.index
}

func (sm *lcsStateMachine) toIndex() int {
	return sm.to.index
}
