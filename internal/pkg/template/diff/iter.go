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

func (i *lcsStateMachine) action() action {
	var action action
	var (
		lcsDone  = i.lcsIndices.index >= len(i.lcsIndices.data)
		fromDone = i.from.index >= len(i.from.data)
		toDone   = i.to.index >= len(i.to.data)
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
		i.currAction = action
		return action
	}
	commonIdx := i.lcsIndices.data[i.lcsIndices.index]
	switch {
	case i.from.index == commonIdx.inA && i.to.index == commonIdx.inB:
		action = actionMatch
	case i.from.index != commonIdx.inA && i.to.index != commonIdx.inB:
		action = actionMod
	case i.from.index != commonIdx.inA:
		// Ex: "a,b,c" -> "c,1,2". When the lcsStateMachine is at (a,c /(b,c), only "c" is common, a,b are considered deleted.
		action = actionDel
	default:
		// Ex: "a,b,c" -> "1,2,a". When the lcsStateMachine is at (a,1) and (a,2), only "a" is common, "1,2" are considered inserted.
		action = actionInsert
	}
	i.currAction = action
	return action
}

func (i *lcsStateMachine) peek() action {
	var lcsIdxOld, fromIdxOld, toIdxOld, actionOld = i.lcsIndices.index, i.from.index, i.to.index, i.currAction
	i.next()
	next := i.action()
	i.lcsIndices.index, i.from.index, i.to.index, i.currAction = lcsIdxOld, fromIdxOld, toIdxOld, actionOld
	return next
}

func (i *lcsStateMachine) next() {
	switch i.currAction {
	case actionMatch:
		i.lcsIndices.index++
		fallthrough
	case actionMod:
		i.from.index++
		i.to.index++
	case actionDel:
		i.from.index++
	case actionInsert:
		i.to.index++
	}
}

func (i *lcsStateMachine) fromItem() yaml.Node {
	return i.from.data[i.from.index]
}

func (i *lcsStateMachine) toItem() yaml.Node {
	return i.to.data[i.to.index]
}

func (i *lcsStateMachine) fromIndex() int {
	return i.from.index
}

func (i *lcsStateMachine) toIndex() int {
	return i.to.index
}
