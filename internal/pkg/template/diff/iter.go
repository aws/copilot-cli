// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import "gopkg.in/yaml.v3"

type action int

const (
	match        = iota
	modification = iota
	deletion     = iota
	insertion    = iota
	done         = iota
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
			action = deletion
		case fromDone:
			action = insertion
		default:
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
		action = deletion
	default:
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
