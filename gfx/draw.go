package gfx


// Renderer must be initialized first.
// Output a string to terminal screen
func Output(text string) {
	renderer.renderer.Write(text)

}
func RenderAndOutput() {
	Output(Render())
}
