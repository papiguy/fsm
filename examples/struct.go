// +build ignore

package main

import (
	"fmt"
	"github.com/papiguy/fsm"
)

type Door struct {
	To  string
	FSM *fsm.FSM
}

func NewDoor(to string) *Door {
	d := &Door{
		To: to,
	}

	d.FSM = fsm.NewFSM(
		"closed",
		fsm.Events{
			{EvtName: "open", SrcStates: []string{"closed"}, DstStates: "open"},
			{EvtName: "close", SrcStates: []string{"open"}, DstStates: "closed"},
		},
		fsm.Callbacks{
			"enter_state": func(action string, e *fsm.Event) { d.enterState(e) },
		},
	)

	return d
}

func (d *Door) enterState(e *fsm.Event) {
	fmt.Printf("The door to %s is %s\n", d.To, e.Dst)
}

func main() {
	door := NewDoor("heaven")

	err := door.FSM.Event("open")
	if err != nil {
		fmt.Println(err)
	}

	err = door.FSM.Event("close")
	if err != nil {
		fmt.Println(err)
	}
}
