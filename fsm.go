// Copyright (c) 2013 - Max Persson <max@looplab.se>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package fsm implements a finite state machine.
//
// It is heavily based on two FSM implementations:
//
// Javascript Finite State Machine
// https://github.com/jakesgordon/javascript-state-machine
//
// Fysom for Python
// https://github.com/oxplot/fysom (forked at https://github.com/mriehl/fysom)
//
package fsm

import (
	"github.com/emicklei/dot"
	"strings"
	"sync"
)

// transitioner is an interface for the FSM's transition function.
type transitioner interface {
	transition(*FSM) error
}

// FSM is the state machine that holds the current state.
//
// It has to be created with NewFSM to function properly.
type FSM struct {
	allStates map[string]bool

	// current is the state that the FSM is currently in.
	current string

	// transitions maps events and source states to destination states.
	transitions map[eKey]string

	// callbacks maps events and targers to callback functions.
	callbacks map[cKey]Callback

	// transition is the internal transition functions used either directly
	// or when Transition is called in an asynchronous state transition.
	transition func() error
	// transitionerObj calls the FSM's transition() function.
	transitionerObj transitioner

	// stateMu guards access to the current state.
	stateMu sync.RWMutex
	// eventMu guards access to Event() and Transition().
	eventMu sync.Mutex
}

// EventDesc represents an event when initializing the FSM.
//
// The event can have one or more source states that is valid for performing
// the transition. If the FSM is in one of the source states it will end up in
// the specified destination state, calling all defined callbacks as it goes.
type EventDesc struct {
	// EvtName is the event name used when calling for a transition.
	EvtName string

	// SrcStates is a slice of source states that the FSM must be in to perform a
	// state transition.
	SrcStates []string

	// DstStates is the destination state that the FSM will be in if the transition
	// succeds.
	DstStates string
}

const ActionBeforeEvent = "BeforeEvent"
const ActionLeavingState = "LeavingState"
const ActionEnteringState = "EnteringState"
const ActionOnEvent = "OnEvent"
const ActionAfterEvent = "AfterEvent"

// Callback is a function type that callbacks should use. Event is the current
// event info as the callback happens.
type Callback func(string, *Event)

// Events is a shorthand for defining the transition map in NewFSM.
type Events []EventDesc

// Callbacks is a shorthand for defining the callbacks in NewFSM.a
type Callbacks map[string]Callback

// NewFSM constructs a FSM from events and callbacks.
//
// The events and transitions are specified as a slice of Event structs
// specified as Events. Each Event is mapped to one or more internal
// transitions from Event.SrcStates to Event.DstStates.
//
// Callbacks are added as a map specified as Callbacks where the key is parsed
// as the callback event as follows, and called in the same order:
//
// 1. before_<EVENT> - called before event named <EVENT>
//
// 2. before_event - called before all events
//
// 3. leave_<OLD_STATE> - called before leaving <OLD_STATE>
//
// 4. leave_state - called before leaving all states
//
// 5. enter_<NEW_STATE> - called after entering <NEW_STATE>
//
// 6. enter_state - called after entering all states
//
// 7. after_<EVENT> - called after event named <EVENT>
//
// 8. after_event - called after all events
//
// There are also two short form versions for the most commonly used callbacks.
// They are simply the name of the event or state:
//
// 1. <NEW_STATE> - called after entering <NEW_STATE>
//
// 2. <EVENT> - called after event named <EVENT>
//
// If both a shorthand version and a full version is specified it is undefined
// which version of the callback will end up in the internal map. This is due
// to the psuedo random nature of Go maps. No checking for multiple keys is
// currently performed.
func NewFSM(initial string, events []EventDesc, callbacks map[string]Callback) *FSM {
	f := &FSM{
		transitionerObj: &transitionerStruct{},
		current:         initial,
		transitions:     make(map[eKey]string),
		callbacks:       make(map[cKey]Callback),
	}

	// Build transition map and store sets of all events and states.
	allEvents := make(map[string]bool)
	f.allStates = make(map[string]bool)
	for _, e := range events {
		for _, src := range e.SrcStates {
			f.transitions[eKey{e.EvtName, src}] = e.DstStates
			f.allStates[src] = true
			f.allStates[e.DstStates] = true
		}
		allEvents[e.EvtName] = true
	}

	// Map all callbacks to events/states.
	for name, fn := range callbacks {
		var target string
		var callbackType int

		switch {
		case strings.HasPrefix(name, "before_"):
			target = strings.TrimPrefix(name, "before_")
			if target == "event" {
				target = ""
				callbackType = callbackBeforeEvent
			} else if _, ok := allEvents[target]; ok {
				callbackType = callbackBeforeEvent
			}
		case strings.HasPrefix(name, "leave_"):
			target = strings.TrimPrefix(name, "leave_")
			if target == "state" {
				target = ""
				callbackType = callbackLeaveState
			} else if _, ok := f.allStates[target]; ok {
				callbackType = callbackLeaveState
			}
		case strings.HasPrefix(name, "enter_"):
			target = strings.TrimPrefix(name, "enter_")
			if target == "state" {
				target = ""
				callbackType = callbackEnterState
			} else if _, ok := f.allStates[target]; ok {
				callbackType = callbackEnterState
			}
		case strings.HasPrefix(name, "after_"):
			target = strings.TrimPrefix(name, "after_")
			if target == "event" {
				target = ""
				callbackType = callbackAfterEvent
			} else if _, ok := allEvents[target]; ok {
				callbackType = callbackAfterEvent
			}
		default:
			target = name
			if _, ok := f.allStates[target]; ok {
				callbackType = callbackOnState
			} else if _, ok := allEvents[target]; ok {
				callbackType = callbackAfterEvent
			}
		}

		if callbackType != callbackNone {
			f.callbacks[cKey{target, callbackType}] = fn
		}
	}

	return f
}

// Current returns the current state of the FSM.
func (f *FSM) Current() string {
	f.stateMu.RLock()
	defer f.stateMu.RUnlock()
	return f.current
}

// Is returns true if state is the current state.
func (f *FSM) Is(state string) bool {
	f.stateMu.RLock()
	defer f.stateMu.RUnlock()
	return state == f.current
}

// SetState allows the user to move to the given state from current state.
// The call does not trigger any callbacks, if defined.
func (f *FSM) SetState(state string) {
	f.stateMu.Lock()
	defer f.stateMu.Unlock()
	f.current = state
	return
}

// Can returns true if event can occur in the current state.
func (f *FSM) Can(event string) bool {
	f.stateMu.RLock()
	defer f.stateMu.RUnlock()
	_, ok := f.transitions[eKey{event, f.current}]
	return ok && (f.transition == nil)
}

// AvailableTransitions returns a list of transitions avilable in the
// current state.
func (f *FSM) AvailableTransitions() []string {
	f.stateMu.RLock()
	defer f.stateMu.RUnlock()
	var transitions []string
	for key := range f.transitions {
		if key.src == f.current {
			transitions = append(transitions, key.event)
		}
	}
	return transitions
}

// Cannot returns true if event can not occure in the current state.
// It is a convenience method to help code read nicely.
func (f *FSM) Cannot(event string) bool {
	return !f.Can(event)
}

// Event initiates a state transition with the named event.
//
// The call takes a variable number of arguments that will be passed to the
// callback, if defined.
//
// It will return nil if the state change is ok or one of these errors:
//
// - event X inappropriate because previous transition did not complete
//
// - event X inappropriate in current state Y
//
// - event X does not exist
//
// - internal error on state transition
//
// The last error should never occur in this situation and is a sign of an
// internal bug.
func (f *FSM) Event(event string, args ...interface{}) error {
	f.eventMu.Lock()
	defer f.eventMu.Unlock()

	f.stateMu.RLock()
	defer f.stateMu.RUnlock()

	if f.transition != nil {
		return InTransitionError{event}
	}

	dst, ok := f.transitions[eKey{event, f.current}]
	if !ok {
		for ekey := range f.transitions {
			if ekey.event == event {
				return InvalidEventError{event, f.current}
			}
		}
		return UnknownEventError{event}
	}

	e := &Event{f, event, f.current, dst, nil, args, false, false}

	err := f.beforeEventCallbacks(e)
	if err != nil {
		return err
	}

	// Setup the transition, call it later.
	f.transition = func() error {

		dontSendStateCallbacks := false
		if f.current == dst {
			dontSendStateCallbacks = true
		}

		if err = f.onStateCallbacks(e); err != nil {
			return err
		}

		if e.Err != nil {
			return nil
		}

		if e.canceled {
			e.Err = CanceledError{e.Err}
			return nil
		}

		f.stateMu.Lock()
		f.current = dst
		f.stateMu.Unlock()

		if !dontSendStateCallbacks {
			f.enterStateCallbacks(e)
		}
		f.afterEventCallbacks(e)

		return nil
	}

	if f.current != dst {
		if err = f.leaveStateCallbacks(e); err != nil {
			if _, ok := err.(CanceledError); ok {
				f.transition = nil
			}
			return err
		}
	}

	// Perform the rest of the transition, if not asynchronous.
	f.stateMu.RUnlock()
	err = f.doTransition()
	f.stateMu.RLock()

	if err != nil {
		return InternalError{}
	}

	return e.Err
}

// Transition wraps transitioner.transition.
func (f *FSM) Transition() error {
	f.eventMu.Lock()
	defer f.eventMu.Unlock()
	return f.doTransition()
}

// doTransition wraps transitioner.transition.
func (f *FSM) doTransition() error {
	return f.transitionerObj.transition(f)
}

// transitionerStruct is the default implementation of the transitioner
// interface. Other implementations can be swapped in for testing.
type transitionerStruct struct{}

// Transition completes an asynchrounous state change.
//
// The callback for leave_<STATE> must prviously have called Async on its
// event to have initiated an asynchronous state transition.
func (t transitionerStruct) transition(f *FSM) error {
	if f.transition == nil {
		return NotInTransitionError{}
	}
	err := f.transition()
	f.transition = nil
	return err
}

// beforeEventCallbacks calls the before_ callbacks, first the named then the
// general version.
func (f *FSM) beforeEventCallbacks(e *Event) error {
	if fn, ok := f.callbacks[cKey{e.Event, callbackBeforeEvent}]; ok {
		fn(ActionBeforeEvent, e)
		if e.canceled {
			return CanceledError{e.Err}
		}
	}
	if fn, ok := f.callbacks[cKey{"", callbackBeforeEvent}]; ok {
		fn(ActionBeforeEvent, e)
		if e.canceled {
			return CanceledError{e.Err}
		}
	}
	return nil
}

// leaveStateCallbacks calls the leave_ callbacks, first the named then the
// general version.
func (f *FSM) leaveStateCallbacks(e *Event) error {
	if fn, ok := f.callbacks[cKey{f.current, callbackLeaveState}]; ok {
		fn(ActionLeavingState, e)
		if e.canceled {
			return CanceledError{e.Err}
		} else if e.async {
			return AsyncError{e.Err}
		}
	}
	if fn, ok := f.callbacks[cKey{"", callbackLeaveState}]; ok {
		fn(ActionLeavingState, e)
		if e.canceled {
			return CanceledError{e.Err}
		} else if e.async {
			return AsyncError{e.Err}
		}
	}
	return nil
}

// enterStateCallbacks calls the enter_ callbacks, first the named then the
// general version.
func (f *FSM) enterStateCallbacks(e *Event) {
	if fn, ok := f.callbacks[cKey{f.current, callbackEnterState}]; ok {
		fn(ActionEnteringState, e)
	}

	if fn, ok := f.callbacks[cKey{f.current, callbackOnState}]; ok {
		fn(ActionEnteringState, e)
	}

	if fn, ok := f.callbacks[cKey{"", callbackEnterState}]; ok {
		fn(ActionEnteringState, e)
	}
}

func (f *FSM) onStateCallbacks(e *Event) error {
	if fn, ok := f.callbacks[cKey{f.current, callbackOnState}]; ok {
		fn(ActionOnEvent, e)
	}
	if fn, ok := f.callbacks[cKey{"", callbackOnState}]; ok {
		fn(ActionOnEvent, e)
	}

	return nil
}

// afterEventCallbacks calls the after_ callbacks, first the named then the
// general version.
func (f *FSM) afterEventCallbacks(e *Event) {
	if fn, ok := f.callbacks[cKey{e.Event, callbackAfterEvent}]; ok {
		fn(ActionAfterEvent, e)
	}
	if fn, ok := f.callbacks[cKey{"", callbackAfterEvent}]; ok {
		fn(ActionAfterEvent, e)
	}
}

func (f *FSM) GetDotRep(name string) string {

	g := dot.NewGraph(dot.Directed)
	g.ID(name)
	g.Label(name)
	nodes := make(map[string]dot.Node, len(f.allStates))

	g.Attr("rankdir", "TB")
	g.Attr("splines", "true")
	g.Attr("overlap", "false")
	g.Attr("ratio", "auto")
	g.Attr("ranksep", ".75")
	g.Attr("concentrate", "false")
	g.Attr("ordering", "out")

	nodes[f.current] = g.Node(f.current)
	nodes[f.current].Attr("shape", "Mrecord")
	nodes[f.current].Attr("color", "black")
	nodes[f.current].Attr("fixedsize","true")
	nodes[f.current].Attr("width","2.5")

	for state, _ := range f.allStates {
		if state == f.current {
			continue
		}
		nodes[state] = g.Node(state)
		nodes[state].Attr("shape", "circle")
		nodes[state].Attr("color", "black")
		nodes[state].Attr("fixedsize","true")
		nodes[state].Attr("width","1.5")
	}



	for ekey, destination := range f.transitions {
		g.Edge(nodes[ekey.src], nodes[destination], ekey.event).Attr("color", "blue")
	}


	return g.String()
}

const (
	callbackNone int = iota
	callbackBeforeEvent
	callbackLeaveState
	callbackEnterState
	callbackOnState
	callbackAfterEvent
)

// cKey is a struct key used for keeping the callbacks mapped to a target.
type cKey struct {
	// target is either the name of a state or an event depending on which
	// callback type the key refers to. It can also be "" for a non-targeted
	// callback like before_event.
	target string

	// callbackType is the situation when the callback will be run.
	callbackType int
}

// eKey is a struct key used for storing the transition map.
type eKey struct {
	// event is the name of the event that the keys refers to.
	event string

	// src is the source from where the event can transition.
	src string
}
