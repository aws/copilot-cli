// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import "gopkg.in/yaml.v3"

type action int

const (
	match        action = iota
	modification        = iota
	deletion            = iota
	insertion           = iota
	done                = iota
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
			action = done
		case toDone:
			// Ex: "a,d,e" -> "a". When the inspector moves to "d" in "from", both common and "to" are done, and "d,e" are considered deleted.
			action = deletion
		case fromDone:
			// Ex: "a" -> "a,d,e". When the inspector moves to "d" in "to", both common and "from" are done, and "d,e" are considered to be inserted.
			action = insertion
		default:
			// Ex: "a,b" -> "a,c". When the inspector moves to index 1 of both list, common is done, and b is modified into c.
			action = modification
		}
		i.currAction = action
		return action
	}
	commonIdx := i.lcsIndices.data[i.lcsIndices.index]
	switch {
	case i.from.index == commonIdx.inA && i.to.index == commonIdx.inB:
		action = match
	case i.from.index != commonIdx.inA && i.to.index != commonIdx.inB:
		action = modification
	case i.from.index != commonIdx.inA:
		// Ex: "a,b,c" -> "c,1,2". When the inspector is at (a,c /(b,c), only "c" is common, a,b are considered deleted.
		action = deletion
	default:
		// Ex: "a,b,c" -> "1,2,a". When the inspector is at (a,1) and (a,2), only "a" is common, "1,2" are considered inserted.
		action = insertion
	}
	i.currAction = action
	return action
}

func (i *inspector) proceed() {
	switch i.currAction {
	case match:
		i.lcsIndices.index++
		fallthrough
	case modification:
		i.from.index++
		i.to.index++
	case deletion:
		i.from.index++
	case insertion:
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
