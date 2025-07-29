package main

import (
	"game/gfx"
	"game/tea"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
)

var gf string

func main() {
	close := gfx.InitRenderer(200, 60)
	defer close()

	for msg := range gfx.Messages() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c":
				return
			case " ":
				DrawShit()
				gfx.RenderAndOutput()
			}
		}
	}

}

var rectX int

func DrawShit() {
	screenW, screenH := gfx.GetScreenSize()

	rectW := 20
	rectH := 10
	y := screenH / 2

	bg := color.RGBA{0, 0, 0, 255}
	fg := color.RGBA{255, 0, 0, 255}

	fillScreen(bg)
	drawRect(rectX, y, rectW, rectH, fg)

	// move rectangle forward, and loop back if it reaches the edge
	rectX++
	if rectX > screenW-rectW {
		rectX = 0
	}
}

// drawRect draws a filled rectangle at (x, y) with given width, height, and color
func drawRect(x, y, w, h int, col color.RGBA) {
	for dx := 0; dx < w; dx++ {
		for dy := 0; dy < h; dy++ {
			gfx.DrawPixel(x+dx, y+dy, col)
		}
	}
}

// fillScreen fills the entire screen with a background color
func fillScreen(col color.RGBA) {
	width, height := gfx.GetScreenSize()
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			gfx.DrawPixel(x, y, col)
		}
	}
}
