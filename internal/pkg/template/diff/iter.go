// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import "gopkg.in/yaml.v3"

type action int

const (
	actionMatch action = iota
	actionMod
	actionDel
	actionInsert
	actonDone
)

type tracker[T any] struct {
	index int
	data  []T
}

func newInspector(fromSeq, toSeq []yaml.Node, lcsIndices []lcsIndex) inspector {
	return inspector{
		from:       tracker[yaml.Node]{data: fromSeq},
		to:         tracker[yaml.Node]{data: toSeq},
		lcsIndices: tracker[lcsIndex]{data: lcsIndices},
	}
}

type inspector struct {
	from       tracker[yaml.Node]
	to         tracker[yaml.Node]
	lcsIndices tracker[lcsIndex]
	currAction action
}

func (i *inspector) inspect() action {
	var action action
	var (
		commonDone = i.lcsIndices.index >= len(i.lcsIndices.data)
		fromDone   = i.from.index >= len(i.from.data)
		toDone     = i.to.index >= len(i.to.data)
	)
	if commonDone {
		switch {
		case fromDone && toDone:
			action = actonDone
		case toDone:
			// Ex: "a,d,e" -> "a". When the inspector moves to "d" in "from", both common and "to" are done, and "d,e" are considered deleted.
			action = actionDel
		case fromDone:
			// Ex: "a" -> "a,d,e". When the inspector moves to "d" in "to", both common and "from" are done, and "d,e" are considered to be inserted.
			action = actionInsert
		default:
			// Ex: "a,b" -> "a,c". When the inspector moves to index 1 of both list, common is done, and b is modified into c.
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
		// Ex: "a,b,c" -> "c,1,2". When the inspector is at (a,c /(b,c), only "c" is common, a,b are considered deleted.
		action = actionDel
	default:
		// Ex: "a,b,c" -> "1,2,a". When the inspector is at (a,1) and (a,2), only "a" is common, "1,2" are considered inserted.
		action = actionInsert
	}
	i.currAction = action
	return action
}

func (i *inspector) proceed() {
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

func (i *inspector) fromItem() yaml.Node {
	return i.from.data[i.from.index]
}

func (i *inspector) toItem() yaml.Node {
	return i.to.data[i.to.index]
}

func (i *inspector) fromIndex() int {
	return i.from.index
}

func (i *inspector) toIndex() int {
	return i.to.index
}
