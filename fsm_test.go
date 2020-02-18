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

package fsm

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"
)

type fakeTransitionerObj struct {
}

func (t fakeTransitionerObj) transition(f *FSM) error {
	return &InternalError{}
}

func TestSameState(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "start"},
		},
		Callbacks{},
	)
	fsm.Event("run")
	if fsm.Current() != "start" {
		t.Error("expected state to be 'start'")
	}
}

func TestSetState(t *testing.T) {
	fsm := NewFSM(
		"walking",
		Events{
			{EvtName: "walk", SrcStates: []string{"start"}, DstStates: "walking"},
		},
		Callbacks{},
	)
	fsm.SetState("start")
	if fsm.Current() != "start" {
		t.Error("expected state to be 'walking'")
	}
	err := fsm.Event("walk")
	if err != nil {
		t.Error("transition is expected no error")
	}
}

func TestBadTransition(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "running"},
		},
		Callbacks{},
	)
	fsm.transitionerObj = new(fakeTransitionerObj)
	err := fsm.Event("run")
	if err == nil {
		t.Error("bad transition should give an error")
	}
}

func TestInappropriateEvent(t *testing.T) {
	fsm := NewFSM(
		"closed",
		Events{
			{EvtName: "open", SrcStates: []string{"closed"}, DstStates: "open"},
			{EvtName: "close", SrcStates: []string{"open"}, DstStates: "closed"},
		},
		Callbacks{},
	)
	err := fsm.Event("close")
	if e, ok := err.(InvalidEventError); !ok && e.Event != "close" && e.State != "closed" {
		t.Error("expected 'InvalidEventError' with correct state and event")
	}
}

func TestInvalidEvent(t *testing.T) {
	fsm := NewFSM(
		"closed",
		Events{
			{EvtName: "open", SrcStates: []string{"closed"}, DstStates: "open"},
			{EvtName: "close", SrcStates: []string{"open"}, DstStates: "closed"},
		},
		Callbacks{},
	)
	err := fsm.Event("lock")
	if e, ok := err.(UnknownEventError); !ok && e.Event != "close" {
		t.Error("expected 'UnknownEventError' with correct event")
	}
}

func TestMultipleSources(t *testing.T) {
	fsm := NewFSM(
		"one",
		Events{
			{EvtName: "first", SrcStates: []string{"one"}, DstStates: "two"},
			{EvtName: "second", SrcStates: []string{"two"}, DstStates: "three"},
			{EvtName: "reset", SrcStates: []string{"one", "two", "three"}, DstStates: "one"},
		},
		Callbacks{},
	)

	fsm.Event("first")
	if fsm.Current() != "two" {
		t.Error("expected state to be 'two'")
	}
	fsm.Event("reset")
	if fsm.Current() != "one" {
		t.Error("expected state to be 'one'")
	}
	fsm.Event("first")
	fsm.Event("second")
	if fsm.Current() != "three" {
		t.Error("expected state to be 'three'")
	}
	fsm.Event("reset")
	if fsm.Current() != "one" {
		t.Error("expected state to be 'one'")
	}
}

func TestMultipleEvents(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "first", SrcStates: []string{"start"}, DstStates: "one"},
			{EvtName: "second", SrcStates: []string{"start"}, DstStates: "two"},
			{EvtName: "reset", SrcStates: []string{"one"}, DstStates: "reset_one"},
			{EvtName: "reset", SrcStates: []string{"two"}, DstStates: "reset_two"},
			{EvtName: "reset", SrcStates: []string{"reset_one", "reset_two"}, DstStates: "start"},
		},
		Callbacks{},
	)

	fsm.Event("first")
	fsm.Event("reset")
	if fsm.Current() != "reset_one" {
		t.Error("expected state to be 'reset_one'")
	}
	fsm.Event("reset")
	if fsm.Current() != "start" {
		t.Error("expected state to be 'start'")
	}

	fsm.Event("second")
	fsm.Event("reset")
	if fsm.Current() != "reset_two" {
		t.Error("expected state to be 'reset_two'")
	}
	fsm.Event("reset")
	if fsm.Current() != "start" {
		t.Error("expected state to be 'start'")
	}
}

func TestGenericCallbacks(t *testing.T) {
	beforeEvent := false
	leaveState := false
	enterState := false
	afterEvent := false

	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"before_event": func(action string, e *Event) {
				beforeEvent = true
			},
			"leave_state": func(action string, e *Event) {
				leaveState = true
			},
			"enter_state": func(action string, e *Event) {
				enterState = true
			},
			"after_event": func(action string, e *Event) {
				afterEvent = true
			},
		},
	)

	fsm.Event("run")
	if !(beforeEvent && leaveState && enterState && afterEvent) {
		t.Error("expected all callbacks to be called")
	}
}

func TestSpecificCallbacks(t *testing.T) {
	beforeEvent := false
	leaveState := false
	enterState := false
	afterEvent := false

	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"before_run": func(action string, e *Event) {
				beforeEvent = true
			},
			"leave_start": func(action string, e *Event) {
				leaveState = true
			},
			"enter_end": func(action string, e *Event) {
				enterState = true
			},
			"after_run": func(action string, e *Event) {
				afterEvent = true
			},
		},
	)

	fsm.Event("run")
	if !(beforeEvent && leaveState && enterState && afterEvent) {
		t.Error("expected all callbacks to be called")
	}
}

func TestSpecificCallbacksShortform(t *testing.T) {
	enterState := false
	afterEvent := false

	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"end": func(action string, e *Event) {
				enterState = true
			},
			"run": func(action string, e *Event) {
				afterEvent = true
			},
		},
	)

	fsm.Event("run")
	if !(enterState && afterEvent) {
		t.Error("expected all callbacks to be called")
	}
}

func TestBeforeEventWithoutTransition(t *testing.T) {
	beforeEvent := true

	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "dontrun", SrcStates: []string{"start"}, DstStates: "start"},
		},
		Callbacks{
			"before_event": func(action string, e *Event) {
				beforeEvent = true
			},
		},
	)

	err := fsm.Event("dontrun")
	if e, ok := err.(NoTransitionError); !ok && e.Err != nil {
		t.Error("expected 'NoTransitionError' without custom error")
	}

	if fsm.Current() != "start" {
		t.Error("expected state to be 'start'")
	}
	if !beforeEvent {
		t.Error("expected callback to be called")
	}
}

func TestCancelBeforeGenericEvent(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"before_event": func(action string, e *Event) {
				e.Cancel()
			},
		},
	)
	fsm.Event("run")
	if fsm.Current() != "start" {
		t.Error("expected state to be 'start'")
	}
}

func TestCancelBeforeSpecificEvent(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"before_run": func(action string, e *Event) {
				e.Cancel()
			},
		},
	)
	fsm.Event("run")
	if fsm.Current() != "start" {
		t.Error("expected state to be 'start'")
	}
}

func TestCancelLeaveGenericState(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"leave_state": func(action string, e *Event) {
				e.Cancel()
			},
		},
	)
	fsm.Event("run")
	if fsm.Current() != "start" {
		t.Error("expected state to be 'start'")
	}
}

func TestCancelLeaveSpecificState(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"leave_start": func(action string, e *Event) {
				e.Cancel()
			},
		},
	)
	fsm.Event("run")
	if fsm.Current() != "start" {
		t.Error("expected state to be 'start'")
	}
}

func TestCancelWithError(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"before_event": func(action string, e *Event) {
				e.Cancel(fmt.Errorf("error"))
			},
		},
	)
	err := fsm.Event("run")
	if _, ok := err.(CanceledError); !ok {
		t.Error("expected only 'CanceledError'")
	}

	if e, ok := err.(CanceledError); ok && e.Err.Error() != "error" {
		t.Error("expected 'CanceledError' with correct custom error")
	}

	if fsm.Current() != "start" {
		t.Error("expected state to be 'start'")
	}
}

func TestAsyncTransitionGenericState(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"leave_state": func(action string, e *Event) {
				e.Async()
			},
		},
	)
	fsm.Event("run")
	if fsm.Current() != "start" {
		t.Error("expected state to be 'start'")
	}
	fsm.Transition()
	if fsm.Current() != "end" {
		t.Error("expected state to be 'end'")
	}
}

func TestAsyncTransitionSpecificState(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"leave_start": func(action string, e *Event) {
				e.Async()
			},
		},
	)
	fsm.Event("run")
	if fsm.Current() != "start" {
		t.Error("expected state to be 'start'")
	}
	fsm.Transition()
	if fsm.Current() != "end" {
		t.Error("expected state to be 'end'")
	}
}

func TestAsyncTransitionInProgress(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
			{EvtName: "reset", SrcStates: []string{"end"}, DstStates: "start"},
		},
		Callbacks{
			"leave_start": func(action string, e *Event) {
				e.Async()
			},
		},
	)
	fsm.Event("run")
	err := fsm.Event("reset")
	if e, ok := err.(InTransitionError); !ok && e.Event != "reset" {
		t.Error("expected 'InTransitionError' with correct state")
	}
	fsm.Transition()
	fsm.Event("reset")
	if fsm.Current() != "start" {
		t.Error("expected state to be 'start'")
	}
}

func TestAsyncTransitionNotInProgress(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
			{EvtName: "reset", SrcStates: []string{"end"}, DstStates: "start"},
		},
		Callbacks{},
	)
	err := fsm.Transition()
	if _, ok := err.(NotInTransitionError); !ok {
		t.Error("expected 'NotInTransitionError'")
	}
}

func TestCallbackNoError(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"run": func(action string, e *Event) {
			},
		},
	)
	e := fsm.Event("run")
	if e != nil {
		t.Error("expected no error")
	}
}

func TestCallbackError(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"run": func(action string, e *Event) {
				e.Err = fmt.Errorf("error")
			},
		},
	)
	e := fsm.Event("run")
	if e.Error() != "error" {
		t.Error("expected error to be 'error'")
	}
}

func TestCallbackArgs(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"run": func(action string, e *Event) {
				if len(e.Args) != 1 {
					t.Error("too few arguments")
				}
				arg, ok := e.Args[0].(string)
				if !ok {
					t.Error("not a string argument")
				}
				if arg != "test" {
					t.Error("incorrect argument")
				}
			},
		},
	)
	fsm.Event("run", "test")
}

func TestNoDeadLock(t *testing.T) {
	var fsm *FSM
	fsm = NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"run": func(action string, e *Event) {
				fsm.Current() // Should not result in a panic / deadlock
			},
		},
	)
	fsm.Event("run")
}

func TestThreadSafetyRaceCondition(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"run": func(action string, e *Event) {
			},
		},
	)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = fsm.Current()
	}()
	fsm.Event("run")
	wg.Wait()
}

func TestDoubleTransition(t *testing.T) {
	var fsm *FSM
	var wg sync.WaitGroup
	wg.Add(2)
	fsm = NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "end"},
		},
		Callbacks{
			"before_run": func(action string, e *Event) {
				wg.Done()
				// Imagine a concurrent event coming in of the same type while
				// the data access mutex is unlocked because the current transition
				// is running its event callbacks, getting around the "active"
				// transition checks
				if len(e.Args) == 0 {
					// Must be concurrent so the test may pass when we add a mutex that synchronizes
					// calls to Event(...). It will then fail as an inappropriate transition as we
					// have changed state.
					go func() {
						if err := fsm.Event("run", "second run"); err != nil {
							fmt.Println(err)
							wg.Done() // It should fail, and then we unfreeze the test.
						}
					}()
					time.Sleep(20 * time.Millisecond)
				} else {
					panic("Was able to reissue an event mid-transition")
				}
			},
		},
	)
	if err := fsm.Event("run"); err != nil {
		fmt.Println(err)
	}
	wg.Wait()
}

func TestNoTransition(t *testing.T) {
	fsm := NewFSM(
		"start",
		Events{
			{EvtName: "run", SrcStates: []string{"start"}, DstStates: "start"},
		},
		Callbacks{},
	)
	err := fsm.Event("run")
	if err == nil {
		return
	}
	if _, ok := err.(NoTransitionError); ok {
		t.Error("expected 'NoTransitionError'")
	}
}

func ExampleNewFSM() {
	fsm := NewFSM(
		"green",
		Events{
			{EvtName: "warn", SrcStates: []string{"green"}, DstStates: "yellow"},
			{EvtName: "panic", SrcStates: []string{"yellow"}, DstStates: "red"},
			{EvtName: "panic", SrcStates: []string{"green"}, DstStates: "red"},
			{EvtName: "calm", SrcStates: []string{"red"}, DstStates: "yellow"},
			{EvtName: "clear", SrcStates: []string{"yellow"}, DstStates: "green"},
		},
		Callbacks{
			"before_warn": func(action string, e *Event) {
				fmt.Println("before_warn")
			},
			"before_event": func(action string, e *Event) {
				fmt.Println("before_event")
			},
			"leave_green": func(action string, e *Event) {
				fmt.Println("leave_green")
			},
			"leave_state": func(action string, e *Event) {
				fmt.Println("leave_state")
			},
			"enter_yellow": func(action string, e *Event) {
				fmt.Println("enter_yellow")
			},
			"enter_state": func(action string, e *Event) {
				fmt.Println("enter_state")
			},
			"after_warn": func(action string, e *Event) {
				fmt.Println("after_warn")
			},
			"after_event": func(action string, e *Event) {
				fmt.Println("after_event")
			},
		},
	)
	fmt.Println(fsm.Current())
	err := fsm.Event("warn")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(fsm.Current())
	// Output:
	// green
	// before_warn
	// before_event
	// leave_green
	// leave_state
	// enter_yellow
	// enter_state
	// after_warn
	// after_event
	// yellow
}

func ExampleFSM_Current() {
	fsm := NewFSM(
		"closed",
		Events{
			{EvtName: "open", SrcStates: []string{"closed"}, DstStates: "open"},
			{EvtName: "close", SrcStates: []string{"open"}, DstStates: "closed"},
		},
		Callbacks{},
	)
	fmt.Println(fsm.Current())
	// Output: closed
}

func ExampleFSM_Is() {
	fsm := NewFSM(
		"closed",
		Events{
			{EvtName: "open", SrcStates: []string{"closed"}, DstStates: "open"},
			{EvtName: "close", SrcStates: []string{"open"}, DstStates: "closed"},
		},
		Callbacks{},
	)
	fmt.Println(fsm.Is("closed"))
	fmt.Println(fsm.Is("open"))
	// Output:
	// true
	// false
}

func ExampleFSM_Can() {
	fsm := NewFSM(
		"closed",
		Events{
			{EvtName: "open", SrcStates: []string{"closed"}, DstStates: "open"},
			{EvtName: "close", SrcStates: []string{"open"}, DstStates: "closed"},
		},
		Callbacks{},
	)
	fmt.Println(fsm.Can("open"))
	fmt.Println(fsm.Can("close"))
	// Output:
	// true
	// false
}

func ExampleFSM_AvailableTransitions() {
	fsm := NewFSM(
		"closed",
		Events{
			{EvtName: "open", SrcStates: []string{"closed"}, DstStates: "open"},
			{EvtName: "close", SrcStates: []string{"open"}, DstStates: "closed"},
			{EvtName: "kick", SrcStates: []string{"closed"}, DstStates: "broken"},
		},
		Callbacks{},
	)
	// sort the results ordering is consistent for the output checker
	transitions := fsm.AvailableTransitions()
	sort.Strings(transitions)
	fmt.Println(transitions)
	// Output:
	// [kick open]
}

func ExampleFSM_Cannot() {
	fsm := NewFSM(
		"closed",
		Events{
			{EvtName: "open", SrcStates: []string{"closed"}, DstStates: "open"},
			{EvtName: "close", SrcStates: []string{"open"}, DstStates: "closed"},
		},
		Callbacks{},
	)
	fmt.Println(fsm.Cannot("open"))
	fmt.Println(fsm.Cannot("close"))
	// Output:
	// false
	// true
}

func ExampleFSM_Event() {
	fsm := NewFSM(
		"closed",
		Events{
			{EvtName: "open", SrcStates: []string{"closed"}, DstStates: "open"},
			{EvtName: "close", SrcStates: []string{"open"}, DstStates: "closed"},
		},
		Callbacks{},
	)
	fmt.Println(fsm.Current())
	err := fsm.Event("open")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(fsm.Current())
	err = fsm.Event("close")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(fsm.Current())
	// Output:
	// closed
	// open
	// closed
}

func ExampleFSM_Transition() {
	fsm := NewFSM(
		"closed",
		Events{
			{EvtName: "open", SrcStates: []string{"closed"}, DstStates: "open"},
			{EvtName: "close", SrcStates: []string{"open"}, DstStates: "closed"},
		},
		Callbacks{
			"leave_closed": func(action string, e *Event) {
				e.Async()
			},
		},
	)
	err := fsm.Event("open")
	if e, ok := err.(AsyncError); !ok && e.Err != nil {
		fmt.Println(err)
	}
	fmt.Println(fsm.Current())
	err = fsm.Transition()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(fsm.Current())
	// Output:
	// closed
	// open
}

func ExampleFSM_OnStateTransition() {
	onStateCalled := false
	fsm := NewFSM(
		"closed",
		Events{
			{EvtName: "open", SrcStates: []string{"closed"}, DstStates: "open"},
			{EvtName: "close", SrcStates: []string{"open"}, DstStates: "closed"},
		},
		Callbacks{
			"closed": func(action string, e *Event) {
				if e.Event == "open" {
					onStateCalled = true
				}
			},
		},
	)
	fmt.Println(fsm.Current())
	err := fsm.Event("open")

	if e, ok := err.(AsyncError); !ok && e.Err != nil {
		fmt.Println(err)
	}

	fmt.Println(fsm.Current())
	// Output:
	// closed
	// open
	if !(onStateCalled) {
		fmt.Println("expected all callbacks to be called")
	}
}

func ExampleFSM_OnStateTransitionCancelled() {
	fsm := NewFSM(
		"closed",
		Events{
			{EvtName: "open", SrcStates: []string{"closed"}, DstStates: "open"},
			{EvtName: "close", SrcStates: []string{"open"}, DstStates: "closed"},
		},
		Callbacks{
			"closed": func(action string, e *Event) {
				if e.Event == "open" {
					e.canceled = true
				}
			},
		},
	)
	fmt.Println(fsm.Current())
	err := fsm.Event("open")

	if err == nil {
		fmt.Println("Expected the state to be canceled")
	}

	fmt.Println(fsm.Current())
	// Output:
	// closed
	// closed
}

func ExampleFSM_MultipleEventOnSameState() {
	counter := 0
	fsm := NewFSM(
		"Idle",
		Events{
			{EvtName: "call", SrcStates: []string{"Idle"}, DstStates: "CallInProgress"},
			{EvtName: "talking", SrcStates: []string{"CallInProgress"}, DstStates: "CallInProgress"},
			{EvtName: "Done", SrcStates: []string{"CallInProgress"}, DstStates: "Idle"},
		},
		Callbacks{
			"Idle": func(action string, e *Event) {
				if action != ActionOnEvent {
					return
				}
				if e.Event == "call" {
					fmt.Println("Taking call")
				}
			},
			"CallInProgress": func(action string, e *Event) {
				if action != ActionOnEvent {
					return
				}
				if e.Event == "talking" {
					counter = counter + 1
					fmt.Println("on call")
				} else if e.Event == "Done" {
					fmt.Println("Call done")
				}
			},
		},
	)
	fmt.Println(fsm.Current())
	err := fsm.Event("call")

	if err != nil {
		fmt.Println(err)
	}

	err = fsm.Event("talking")

	if err != nil {
		fmt.Println(err)
	}

	err = fsm.Event("Done")

	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(counter)

	// Output:
	// Idle
	// Taking call
	// on call
	// Call done
	// 1
}

func ExampleFSM_OnStateTransitionSameEvent() {
	fsm := NewFSM(
		"state1",
		Events{
			{EvtName: "event1", SrcStates: []string{"state1"}, DstStates: "state2"},
			{EvtName: "event1", SrcStates: []string{"state2"}, DstStates: "state2"},
			{EvtName: "event2", SrcStates: []string{"state2"}, DstStates: "state3"},
			{EvtName: "event2", SrcStates: []string{"state3"}, DstStates: "state3"},
		},
		Callbacks{
			"state1": func(action string, e *Event) {
				if action != ActionOnEvent {
					return
				}
				if e.Event == "event1" {
					fmt.Println("state1 -> event1 received")
				}
			},
			"state2": func(action string, e *Event) {
				if action != ActionOnEvent {
					return
				}

				if e.Event == "event1" {
					fmt.Println("state2 -> event1 received")
				}
				if e.Event == "event2" {
					fmt.Println("state2 -> event2 received")
				}
			},
			"state3": func(action string, e *Event) {
				if action != ActionOnEvent {
					return
				}

				if e.Event == "event2" {
					fmt.Println("state3 -> event2 received")
				}
			},
		},
	)

	fsm.Event("event1")
	fmt.Println(fsm.Current())
	fsm.Event("event1")
	fmt.Println(fsm.Current())
	fsm.Event("event1")
	fmt.Println(fsm.Current())
	fsm.Event("event1")
	fmt.Println(fsm.Current())
	fsm.Event("event2")
	fmt.Println(fsm.Current())
	fsm.Event("event2")
	fmt.Println(fsm.Current())
	fsm.Event("event2")
	fmt.Println(fsm.Current())

	// Output:
	// state1 -> event1 received
	//state2
	//state2 -> event1 received
	//state2
	//state2 -> event1 received
	//state2
	//state2 -> event1 received
	//state2
	//state2 -> event2 received
	//state3
	//state3 -> event2 received
	//state3
	//state3 -> event2 received
	//state3
}
