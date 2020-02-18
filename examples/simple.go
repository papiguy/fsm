// +build ignore

package main

import (
	"fmt"
	"github.com/papiguy/fsm"
)

func main() {
	fsm := fsm.NewFSM(
		"closed",
		fsm.Events{
			{EvtName: "open", SrcStates: []string{"closed"}, DstStates: "open"},
			{EvtName: "close", SrcStates: []string{"open"}, DstStates: "closed"},
		},
		fsm.Callbacks{},
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
}
