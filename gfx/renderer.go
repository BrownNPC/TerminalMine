package gfx

import (
	"context"
	"game/tea"
	"os"

	"github.com/fogleman/fauxgl"
	"golang.org/x/term"
)

var renderer = struct {
	renderer tea.Renderer
	fps      int
	glctx    *fauxgl.Context
}{}

var messages = NewRingBuffer[tea.Msg](50)

func Gl() *fauxgl.Context { return renderer.glctx }

func InitRenderer(WidthAndHeight int, fps int) (close func()) {
	renderer.glctx = fauxgl.NewContext(WidthAndHeight, WidthAndHeight)

	ctx, cancel := context.WithCancel(context.Background())
	msgs := make(chan tea.Msg)
	go tea.ReadInputs(ctx, msgs, os.Stdin)
	go func() {
		select {
		case <-ctx.Done():
			return
		default:
			for msg := range msgs {
				messages.Append(msg)
			}
		}
	}()

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	tm := term.NewTerminal(os.Stdin, "")
	r := tea.NewRenderer(tm, false, fps)
	renderer.renderer = r
	renderer.fps = fps
	r.HideCursor()
	r.EnableMouseAllMotion()
	r.EnableMouseSGRMode()
	renderer.renderer.EnterAltScreen()
	r.Start()

	return func() {
		cancel()
		r.DisableMouseAllMotion()
		r.DisableMouseSGRMode()
		r.Stop()
		r.ShowCursor()
		term.Restore(int(os.Stdin.Fd()), oldState)
	}
}
