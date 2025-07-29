package tea

// Renderer is the interface for Bubble Tea renderers.
type Renderer interface {
	// Start the renderer.
	Start()

	// Stop the renderer, but render the final frame in the buffer, if any.
	Stop()

	// Stop the renderer without doing any final rendering.
	Kill()

	// Write a frame to the renderer. The renderer can Write this data to
	// output at its discretion.
	Write(string)

	// Request a full re-render. Note that this will not trigger a render
	// immediately. Rather, this method causes the next render to be a full
	// Repaint. Because of this, it's safe to call this method multiple times
	// in succession.
	Repaint()

	// Clears the terminal.
	ClearScreen()

	// Whether or not the alternate screen buffer is enabled.
	AltScreen() bool
	// Enable the alternate screen buffer.
	EnterAltScreen()
	// Disable the alternate screen buffer.
	ExitAltScreen()

	// Show the cursor.
	ShowCursor()
	// Hide the cursor.
	HideCursor()

	// EnableMouseCellMotion enables mouse click, release, wheel and motion
	// events if a mouse button is pressed (i.e., drag events).
	EnableMouseCellMotion()

	// DisableMouseCellMotion disables Mouse Cell Motion tracking.
	DisableMouseCellMotion()

	// EnableMouseAllMotion enables mouse click, release, wheel and motion
	// events, regardless of whether a mouse button is pressed. Many modern
	// terminals support this, but not all.
	EnableMouseAllMotion()

	// DisableMouseAllMotion disables All Motion mouse tracking.
	DisableMouseAllMotion()

	// EnableMouseSGRMode enables mouse extended mode (SGR).
	EnableMouseSGRMode()

	// DisableMouseSGRMode disables mouse extended mode (SGR).
	DisableMouseSGRMode()

	// EnableBracketedPaste enables bracketed paste, where characters
	// inside the input are not interpreted when pasted as a whole.
	EnableBracketedPaste()

	// DisableBracketedPaste disables bracketed paste.
	DisableBracketedPaste()

	// BracketedPasteActive reports whether bracketed paste mode is
	// currently enabled.
	BracketedPasteActive() bool

	// SetWindowTitle sets the terminal window title.
	SetWindowTitle(string)

	// ReportFocus returns whether reporting focus events is enabled.
	ReportFocus() bool

	// EnableReportFocus reports focus events to the program.
	EnableReportFocus()

	// DisableReportFocus stops reporting focus events to the program.
	DisableReportFocus()

	// ResetLinesRendered ensures exec output remains on screen on exit
	ResetLinesRendered()
}

// repaintMsg forces a full repaint.
type repaintMsg struct{}
