package tea

type nilRenderer struct{}

func (n nilRenderer) Start()                     {}
func (n nilRenderer) Stop()                      {}
func (n nilRenderer) Kill()                      {}
func (n nilRenderer) Write(_ string)             {}
func (n nilRenderer) Repaint()                   {}
func (n nilRenderer) ClearScreen()               {}
func (n nilRenderer) AltScreen() bool            { return false }
func (n nilRenderer) EnterAltScreen()            {}
func (n nilRenderer) ExitAltScreen()             {}
func (n nilRenderer) ShowCursor()                {}
func (n nilRenderer) HideCursor()                {}
func (n nilRenderer) EnableMouseCellMotion()     {}
func (n nilRenderer) DisableMouseCellMotion()    {}
func (n nilRenderer) EnableMouseAllMotion()      {}
func (n nilRenderer) DisableMouseAllMotion()     {}
func (n nilRenderer) EnableBracketedPaste()      {}
func (n nilRenderer) DisableBracketedPaste()     {}
func (n nilRenderer) EnableMouseSGRMode()        {}
func (n nilRenderer) DisableMouseSGRMode()       {}
func (n nilRenderer) BracketedPasteActive() bool { return false }
func (n nilRenderer) SetWindowTitle(_ string)    {}
func (n nilRenderer) ReportFocus() bool          { return false }
func (n nilRenderer) EnableReportFocus()         {}
func (n nilRenderer) DisableReportFocus()        {}
func (n nilRenderer) ResetLinesRendered()        {}
