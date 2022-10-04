package main

import (
	"fmt"
	"time"

	"github.com/micmonay/keybd_event"
)

func selectAll() (err error) {
	var kb keybd_event.KeyBonding

	kb, err = keybd_event.NewKeyBonding()
	if err != nil {
		fmt.Println("KEY ERROR")
	}

	kb.HasCTRL(true)
	kb.SetKeys(keybd_event.VK_A)

	kb.Press()
	time.Sleep(10 * time.Millisecond)
	kb.Release()

	return
}
