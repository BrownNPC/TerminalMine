package tea

import "testing"

func TestNilRenderer(t *testing.T) {
	r := nilRenderer{}
	r.Start()
	r.Stop()
	r.Kill()
	r.Write("a")
	r.Repaint()
	r.EnterAltScreen()
	if r.AltScreen() {
		t.Errorf("altScreen should always return false")
	}
	r.ExitAltScreen()
	r.ClearScreen()
	r.ShowCursor()
	r.HideCursor()
	r.EnableMouseCellMotion()
	r.DisableMouseCellMotion()
	r.EnableMouseAllMotion()
	r.DisableMouseAllMotion()
}
