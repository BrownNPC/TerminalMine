package gfx

import (
	"game/tea"
	"iter"
	"time"
)

// waits till its time to draw a frame and sends down an event
func Messages() iter.Seq[tea.Msg] {
	return func(yield func(tea.Msg) bool) {
		for {
			time.Sleep(time.Second / time.Duration(renderer.fps))

			msg, _ := messages.Pop()

			if !yield(msg) {
				break // stop if downstream doesn't want more
			}
		}
	}
}
