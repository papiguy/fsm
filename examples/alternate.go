// +build ignore

package main

import (
	"fmt"
	"github.com/papiguy/fsm"
)

func main() {
	fsm := fsm.NewFSM(
		"idle",
		fsm.Events{
			{EvtName: "scan", SrcStates: []string{"idle"}, DstStates: "scanning"},
			{EvtName: "working", SrcStates: []string{"scanning"}, DstStates: "scanning"},
			{EvtName: "situation", SrcStates: []string{"scanning"}, DstStates: "scanning"},
			{EvtName: "situation", SrcStates: []string{"idle"}, DstStates: "idle"},
			{EvtName: "finish", SrcStates: []string{"scanning"}, DstStates: "idle"},
		},
		fsm.Callbacks{
			"scan": func(action string, e *fsm.Event) {
				fmt.Println("after_scan: " + e.FSM.Current())
			},
			"working": func(action string, e *fsm.Event) {
				fmt.Println("working: " + e.FSM.Current())
			},
			"situation": func(action string, e *fsm.Event) {
				fmt.Println("situation: " + e.FSM.Current())
			},
			"finish": func(action string, e *fsm.Event) {
				fmt.Println("finish: " + e.FSM.Current())
			},
		},
	)

	fmt.Println(fsm.Current())

	err := fsm.Event("scan")
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("1:" + fsm.Current())

	err = fsm.Event("working")
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("2:" + fsm.Current())

	err = fsm.Event("situation")
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("3:" + fsm.Current())

	err = fsm.Event("finish")
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("4:" + fsm.Current())

}
