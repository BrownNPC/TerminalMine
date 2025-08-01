// Package tea provides a framework for building rich terminal user interfaces
// based on the paradigms of The Elm Architecture. It's well-suited for simple
// and complex terminal applications, either inline, full-window, or a mix of
// both. It's been battle-tested in several large projects and is
// production-ready.
//
// A tutorial is available at https://github.com/charmbracelet/bubbletea/tree/master/tutorials
//
// Example programs can be found at https://github.com/charmbracelet/bubbletea/tree/master/examples
package tea

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/charmbracelet/x/term"
	"github.com/muesli/cancelreader"
	"golang.org/x/sync/errgroup"
)

// ErrProgramPanic is returned by [Program.Run] when the program recovers from a panic.
var ErrProgramPanic = errors.New("program experienced a panic")

// ErrProgramKilled is returned by [Program.Run] when the program gets killed.
var ErrProgramKilled = errors.New("program was killed")

// ErrInterrupted is returned by [Program.Run] when the program get a SIGINT
// signal, or when it receives a [InterruptMsg].
var ErrInterrupted = errors.New("program was interrupted")

// Msg contain data from the result of a IO operation. Msgs trigger the update
// function and, henceforth, the UI.
type Msg interface{}

// Model contains the program's state as well as its core functions.
type Model interface {
	// Init is the first function that will be called. It returns an optional
	// initial command. To not perform an initial command return nil.
	Init() Cmd

	// Update is called when a message is received. Use it to inspect messages
	// and, in response, update the model and/or send a command.
	Update(Msg) (Model, Cmd)

	// View renders the program's UI, which is just a string. The view is
	// rendered after every Update.
	View() string
}

// Cmd is an IO operation that returns a message when it's complete. If it's
// nil it's considered a no-op. Use it for things like HTTP requests, timers,
// saving and loading from disk, and so on.
//
// Note that there's almost never a reason to use a command to send a message
// to another part of your program. That can almost always be done in the
// update function.
type Cmd func() Msg

type inputType int

const (
	defaultInput inputType = iota
	ttyInput
	customInput
)

// String implements the stringer interface for [inputType]. It is intended to
// be used in testing.
func (i inputType) String() string {
	return [...]string{
		"default input",
		"tty input",
		"custom input",
	}[i]
}

// Options to customize the program during its initialization. These are
// generally set with ProgramOptions.
//
// The options here are treated as bits.
type startupOptions int16

func (s startupOptions) has(option startupOptions) bool {
	return s&option != 0
}

const (
	withAltScreen startupOptions = 1 << iota
	withMouseCellMotion
	withMouseAllMotion
	withANSICompressor
	withoutSignalHandler
	// Catching panics is incredibly useful for restoring the terminal to a
	// usable state after a panic occurs. When this is set, Bubble Tea will
	// recover from panics, print the stack trace, and disable raw mode. This
	// feature is on by default.
	withoutCatchPanics
	withoutBracketedPaste
	withReportFocus
)

// channelHandlers manages the series of channels returned by various processes.
// It allows us to wait for those processes to terminate before exiting the
// program.
type channelHandlers []chan struct{}

// Adds a channel to the list of handlers. We wait for all handlers to terminate
// gracefully on shutdown.
func (h *channelHandlers) add(ch chan struct{}) {
	*h = append(*h, ch)
}

// shutdown waits for all handlers to terminate.
func (h channelHandlers) shutdown() {
	var wg sync.WaitGroup
	for _, ch := range h {
		wg.Add(1)
		go func(ch chan struct{}) {
			<-ch
			wg.Done()
		}(ch)
	}
	wg.Wait()
}

// Program is a terminal user interface.
type Program struct {
	initialModel Model

	// handlers is a list of channels that need to be waited on before the
	// program can exit.
	handlers channelHandlers

	// Configuration options that will set as the program is initializing,
	// treated as bits. These options can be set via various ProgramOptions.
	startupOptions startupOptions

	// startupTitle is the title that will be set on the terminal when the
	// program starts.
	startupTitle string

	inputType inputType

	// externalCtx is a context that was passed in via WithContext, otherwise defaulting
	// to ctx.Background() (in case it was not), the internal context is derived from it.
	externalCtx context.Context

	// ctx is the programs's internal context for signalling internal teardown.
	// It is built and derived from the externalCtx in NewProgram().
	ctx    context.Context
	cancel context.CancelFunc

	msgs     chan Msg
	errs     chan error
	finished chan struct{}

	// where to send output, this will usually be os.Stdout.
	output io.Writer
	// ttyOutput is null if output is not a TTY.
	ttyOutput           term.File
	previousOutputState *term.State
	renderer            Renderer

	// the environment variables for the program, defaults to os.Environ().
	environ []string

	// where to read inputs from, this will usually be os.Stdin.
	input io.Reader
	// ttyInput is null if input is not a TTY.
	ttyInput              term.File
	previousTtyInputState *term.State
	cancelReader          cancelreader.CancelReader
	readLoopDone          chan struct{}

	// was the altscreen active before releasing the terminal?
	altScreenWasActive bool
	ignoreSignals      uint32

	bpWasActive bool // was the bracketed paste mode active before releasing the terminal?
	reportFocus bool // was focus reporting active before releasing the terminal?

	filter func(Model, Msg) Msg

	// fps is the frames per second we should set on the renderer, if
	// applicable,
	fps int

	// mouseMode is true if the program should enable mouse mode on Windows.
	mouseMode bool
}

// Quit is a special command that tells the Bubble Tea program to exit.
func Quit() Msg {
	return QuitMsg{}
}

// QuitMsg signals that the program should quit. You can send a [QuitMsg] with
// [Quit].
type QuitMsg struct{}

// Suspend is a special command that tells the Bubble Tea program to suspend.
func Suspend() Msg {
	return SuspendMsg{}
}

// SuspendMsg signals the program should suspend.
// This usually happens when ctrl+z is pressed on common programs, but since
// bubbletea puts the terminal in raw mode, we need to handle it in a
// per-program basis.
//
// You can send this message with [Suspend()].
type SuspendMsg struct{}

// ResumeMsg can be listen to do something once a program is resumed back
// from a suspend state.
type ResumeMsg struct{}

// InterruptMsg signals the program should suspend.
// This usually happens when ctrl+c is pressed on common programs, but since
// bubbletea puts the terminal in raw mode, we need to handle it in a
// per-program basis.
//
// You can send this message with [Interrupt()].
type InterruptMsg struct{}

// Interrupt is a special command that tells the Bubble Tea program to
// interrupt.
func Interrupt() Msg {
	return InterruptMsg{}
}

// NewProgram creates a new Program.
func NewProgram(model Model, opts ...ProgramOption) *Program {
	p := &Program{
		initialModel: model,
		msgs:         make(chan Msg),
	}

	// Apply all options to the program.
	for _, opt := range opts {
		opt(p)
	}

	// A context can be provided with a ProgramOption, but if none was provided
	// we'll use the default background context.
	if p.externalCtx == nil {
		p.externalCtx = context.Background()
	}
	// Initialize context and teardown channel.
	p.ctx, p.cancel = context.WithCancel(p.externalCtx)

	// if no output was set, set it to stdout
	if p.output == nil {
		p.output = os.Stdout
	}

	// if no environment was set, set it to os.Environ()
	if p.environ == nil {
		p.environ = os.Environ()
	}

	return p
}

func (p *Program) handleSignals() chan struct{} {
	ch := make(chan struct{})

	// Listen for SIGINT and SIGTERM.
	//
	// In most cases ^C will not send an interrupt because the terminal will be
	// in raw mode and ^C will be captured as a keystroke and sent along to
	// Program.Update as a KeyMsg. When input is not a TTY, however, ^C will be
	// caught here.
	//
	// SIGTERM is sent by unix utilities (like kill) to terminate a process.
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		defer func() {
			signal.Stop(sig)
			close(ch)
		}()

		for {
			select {
			case <-p.ctx.Done():
				return

			case s := <-sig:
				if atomic.LoadUint32(&p.ignoreSignals) == 0 {
					switch s {
					case syscall.SIGINT:
						p.msgs <- InterruptMsg{}
					default:
						p.msgs <- QuitMsg{}
					}
					return
				}
			}
		}
	}()

	return ch
}

// handleResize handles terminal resize events.
func (p *Program) handleResize() chan struct{} {
	ch := make(chan struct{})

	if p.ttyOutput != nil {
		// Get the initial terminal size and send it to the program.
		go p.checkResize()

		// Listen for window resizes.
		go p.listenForResize(ch)
	} else {
		close(ch)
	}

	return ch
}

// handleCommands runs commands in a goroutine and sends the result to the
// program's message channel.
func (p *Program) handleCommands(cmds chan Cmd) chan struct{} {
	ch := make(chan struct{})

	go func() {
		defer close(ch)

		for {
			select {
			case <-p.ctx.Done():
				return

			case cmd := <-cmds:
				if cmd == nil {
					continue
				}

				// Don't wait on these goroutines, otherwise the shutdown
				// latency would get too large as a Cmd can run for some time
				// (e.g. tick commands that sleep for half a second). It's not
				// possible to cancel them so we'll have to leak the goroutine
				// until Cmd returns.
				go func() {
					// Recover from panics.
					if !p.startupOptions.has(withoutCatchPanics) {
						defer func() {
							if r := recover(); r != nil {
								p.recoverFromGoPanic(r)
							}
						}()
					}

					msg := cmd() // this can be long.
					p.Send(msg)
				}()
			}
		}
	}()

	return ch
}

func (p *Program) disableMouse() {
	p.renderer.DisableMouseCellMotion()
	p.renderer.DisableMouseAllMotion()
	p.renderer.DisableMouseSGRMode()
}

// eventLoop is the central message loop. It receives and handles the default
// Bubble Tea messages, update the model and triggers redraws.
func (p *Program) eventLoop(model Model, cmds chan Cmd) (Model, error) {
	for {
		select {
		case <-p.ctx.Done():
			return model, nil

		case err := <-p.errs:
			return model, err

		case msg := <-p.msgs:
			// Filter messages.
			if p.filter != nil {
				msg = p.filter(model, msg)
			}
			if msg == nil {
				continue
			}

			// Handle special internal messages.
			switch msg := msg.(type) {
			case QuitMsg:
				return model, nil

			case InterruptMsg:
				return model, ErrInterrupted

			case SuspendMsg:
				if suspendSupported {
					p.suspend()
				}

			case clearScreenMsg:
				p.renderer.ClearScreen()

			case enterAltScreenMsg:
				p.renderer.EnterAltScreen()

			case exitAltScreenMsg:
				p.renderer.ExitAltScreen()

			case enableMouseCellMotionMsg, enableMouseAllMotionMsg:
				switch msg.(type) {
				case enableMouseCellMotionMsg:
					p.renderer.EnableMouseCellMotion()
				case enableMouseAllMotionMsg:
					p.renderer.EnableMouseAllMotion()
				}
				// mouse mode (1006) is a no-op if the terminal doesn't support it.
				p.renderer.EnableMouseSGRMode()

				// XXX: This is used to enable mouse mode on Windows. We need
				// to reinitialize the cancel reader to get the mouse events to
				// work.
				if runtime.GOOS == "windows" && !p.mouseMode {
					p.mouseMode = true
					p.initCancelReader(true) //nolint:errcheck,gosec
				}

			case disableMouseMsg:
				p.disableMouse()

				// XXX: On Windows, mouse mode is enabled on the input reader
				// level. We need to instruct the input reader to stop reading
				// mouse events.
				if runtime.GOOS == "windows" && p.mouseMode {
					p.mouseMode = false
					p.initCancelReader(true) //nolint:errcheck,gosec
				}

			case showCursorMsg:
				p.renderer.ShowCursor()

			case hideCursorMsg:
				p.renderer.HideCursor()

			case enableBracketedPasteMsg:
				p.renderer.EnableBracketedPaste()

			case disableBracketedPasteMsg:
				p.renderer.DisableBracketedPaste()

			case enableReportFocusMsg:
				p.renderer.EnableReportFocus()

			case disableReportFocusMsg:
				p.renderer.DisableReportFocus()

			case execMsg:
				// NB: this blocks.
				p.exec(msg.cmd, msg.fn)

			case BatchMsg:
				for _, cmd := range msg {
					select {
					case <-p.ctx.Done():
						return model, nil
					case cmds <- cmd:
					}
				}
				continue

			case sequenceMsg:
				go func() {
					// Execute commands one at a time, in order.
					for _, cmd := range msg {
						if cmd == nil {
							continue
						}

						msg := cmd()
						if batchMsg, ok := msg.(BatchMsg); ok {
							g, _ := errgroup.WithContext(p.ctx)
							for _, cmd := range batchMsg {
								cmd := cmd
								g.Go(func() error {
									p.Send(cmd())
									return nil
								})
							}

							//nolint:errcheck,gosec
							g.Wait() // wait for all commands from batch msg to finish
							continue
						}

						p.Send(msg)
					}
				}()

			case setWindowTitleMsg:
				p.SetWindowTitle(string(msg))

			case windowSizeMsg:
				go p.checkResize()
			}

			// Process internal messages for the renderer.
			if r, ok := p.renderer.(*standardRenderer); ok {
				r.handleMessages(msg)
			}

			var cmd Cmd
			model, cmd = model.Update(msg) // run update

			select {
			case <-p.ctx.Done():
				return model, nil
			case cmds <- cmd: // process command (if any)
			}

			p.renderer.Write(model.View()) // send view to renderer
		}
	}
}

// Run initializes the program and runs its event loops, blocking until it gets
// terminated by either [Program.Quit], [Program.Kill], or its signal handler.
// Returns the final model.
func (p *Program) Run() (returnModel Model, returnErr error) {
	p.handlers = channelHandlers{}
	cmds := make(chan Cmd)
	p.errs = make(chan error, 1)

	p.finished = make(chan struct{})
	defer func() {
		close(p.finished)
	}()

	defer p.cancel()

	switch p.inputType {
	case defaultInput:
		p.input = os.Stdin

		// The user has not set a custom input, so we need to check whether or
		// not standard input is a terminal. If it's not, we open a new TTY for
		// input. This will allow things to "just work" in cases where data was
		// piped in or redirected to the application.
		//
		// To disable input entirely pass nil to the [WithInput] program option.
		f, isFile := p.input.(term.File)
		if !isFile {
			break
		}
		if term.IsTerminal(f.Fd()) {
			break
		}

		f, err := openInputTTY()
		if err != nil {
			return p.initialModel, err
		}
		defer f.Close() //nolint:errcheck
		p.input = f

	case ttyInput:
		// Open a new TTY, by request
		f, err := openInputTTY()
		if err != nil {
			return p.initialModel, err
		}
		defer f.Close() //nolint:errcheck
		p.input = f

	case customInput:
		// (There is nothing extra to do.)
	}

	// Handle signals.
	if !p.startupOptions.has(withoutSignalHandler) {
		p.handlers.add(p.handleSignals())
	}

	// Recover from panics.
	if !p.startupOptions.has(withoutCatchPanics) {
		defer func() {
			if r := recover(); r != nil {
				returnErr = fmt.Errorf("%w: %w", ErrProgramKilled, ErrProgramPanic)
				p.recoverFromPanic(r)
			}
		}()
	}

	// If no renderer is set use the standard one.
	if p.renderer == nil {
		p.renderer = NewRenderer(p.output, p.startupOptions.has(withANSICompressor), p.fps)
	}

	// Check if output is a TTY before entering raw mode, hiding the cursor and
	// so on.
	if err := p.initTerminal(); err != nil {
		return p.initialModel, err
	}

	// Honor program startup options.
	if p.startupTitle != "" {
		p.renderer.SetWindowTitle(p.startupTitle)
	}
	if p.startupOptions&withAltScreen != 0 {
		p.renderer.EnterAltScreen()
	}
	if p.startupOptions&withoutBracketedPaste == 0 {
		p.renderer.EnableBracketedPaste()
	}
	if p.startupOptions&withMouseCellMotion != 0 {
		p.renderer.EnableMouseCellMotion()
		p.renderer.EnableMouseSGRMode()
	} else if p.startupOptions&withMouseAllMotion != 0 {
		p.renderer.EnableMouseAllMotion()
		p.renderer.EnableMouseSGRMode()
	}

	// XXX: Should we enable mouse mode on Windows?
	// This needs to happen before initializing the cancel and input reader.
	p.mouseMode = p.startupOptions&withMouseCellMotion != 0 || p.startupOptions&withMouseAllMotion != 0

	if p.startupOptions&withReportFocus != 0 {
		p.renderer.EnableReportFocus()
	}

	// Start the renderer.
	p.renderer.Start()

	// Initialize the program.
	model := p.initialModel
	if initCmd := model.Init(); initCmd != nil {
		ch := make(chan struct{})
		p.handlers.add(ch)

		go func() {
			defer close(ch)

			select {
			case cmds <- initCmd:
			case <-p.ctx.Done():
			}
		}()
	}

	// Render the initial view.
	p.renderer.Write(model.View())

	// Subscribe to user input.
	if p.input != nil {
		if err := p.initCancelReader(false); err != nil {
			return model, err
		}
	}

	// Handle resize events.
	p.handlers.add(p.handleResize())

	// Process commands.
	p.handlers.add(p.handleCommands(cmds))

	// Run event loop, handle updates and draw.
	model, err := p.eventLoop(model, cmds)

	if err == nil && len(p.errs) > 0 {
		err = <-p.errs // Drain a leftover error in case eventLoop crashed
	}

	killed := p.externalCtx.Err() != nil || p.ctx.Err() != nil || err != nil
	if killed {
		if err == nil && p.externalCtx.Err() != nil {
			// Return also as context error the cancellation of an external context.
			// This is the context the user knows about and should be able to act on.
			err = fmt.Errorf("%w: %w", ErrProgramKilled, p.externalCtx.Err())
		} else if err == nil && p.ctx.Err() != nil {
			// Return only that the program was killed (not the internal mechanism).
			// The user does not know or need to care about the internal program context.
			err = ErrProgramKilled
		} else {
			// Return that the program was killed and also the error that caused it.
			err = fmt.Errorf("%w: %w", ErrProgramKilled, err)
		}
	} else {
		// Graceful shutdown of the program (not killed):
		// Ensure we rendered the final state of the model.
		p.renderer.Write(model.View())
	}

	// Restore terminal state.
	p.shutdown(killed)

	return model, err
}

// StartReturningModel initializes the program and runs its event loops,
// blocking until it gets terminated by either [Program.Quit], [Program.Kill],
// or its signal handler. Returns the final model.
//
// Deprecated: please use [Program.Run] instead.
func (p *Program) StartReturningModel() (Model, error) {
	return p.Run()
}

// Start initializes the program and runs its event loops, blocking until it
// gets terminated by either [Program.Quit], [Program.Kill], or its signal
// handler.
//
// Deprecated: please use [Program.Run] instead.
func (p *Program) Start() error {
	_, err := p.Run()
	return err
}

// Send sends a message to the main update function, effectively allowing
// messages to be injected from outside the program for interoperability
// purposes.
//
// If the program hasn't started yet this will be a blocking operation.
// If the program has already been terminated this will be a no-op, so it's safe
// to send messages after the program has exited.
func (p *Program) Send(msg Msg) {
	select {
	case <-p.ctx.Done():
	case p.msgs <- msg:
	}
}

// Quit is a convenience function for quitting Bubble Tea programs. Use it
// when you need to shut down a Bubble Tea program from the outside.
//
// If you wish to quit from within a Bubble Tea program use the Quit command.
//
// If the program is not running this will be a no-op, so it's safe to call
// if the program is unstarted or has already exited.
func (p *Program) Quit() {
	p.Send(Quit())
}

// Kill signals the program to stop immediately and restore the former terminal state.
// The final render that you would normally see when quitting will be skipped.
// [program.Run] returns a [ErrProgramKilled] error.
func (p *Program) Kill() {
	p.cancel()
}

// Wait waits/blocks until the underlying Program finished shutting down.
func (p *Program) Wait() {
	<-p.finished
}

// shutdown performs operations to free up resources and restore the terminal
// to its original state. It is called once at the end of the program's lifetime.
//
// This method should not be called to signal the program to be killed/shutdown.
// Doing so can lead to race conditions with the eventual call at the program's end.
// As alternatives, the [Quit] or [Kill] convenience methods should be used instead.
func (p *Program) shutdown(kill bool) {
	p.cancel()

	// Wait for all handlers to finish.
	p.handlers.shutdown()

	// Check if the cancel reader has been setup before waiting and closing.
	if p.cancelReader != nil {
		// Wait for input loop to finish.
		if p.cancelReader.Cancel() {
			if !kill {
				p.waitForReadLoop()
			}
		}
		_ = p.cancelReader.Close()
	}

	if p.renderer != nil {
		if kill {
			p.renderer.Kill()
		} else {
			p.renderer.Stop()
		}
	}

	_ = p.restoreTerminalState()
}

// recoverFromPanic recovers from a panic, prints the stack trace, and restores
// the terminal to a usable state.
func (p *Program) recoverFromPanic(r interface{}) {
	select {
	case p.errs <- ErrProgramPanic:
	default:
	}
	p.shutdown(true) // Ok to call here, p.Run() cannot do it anymore.
	fmt.Printf("Caught panic:\n\n%s\n\nRestoring terminal...\n\n", r)
	debug.PrintStack()
}

// recoverFromGoPanic recovers from a goroutine panic, prints a stack trace and
// signals for the program to be killed and terminal restored to a usable state.
func (p *Program) recoverFromGoPanic(r interface{}) {
	select {
	case p.errs <- ErrProgramPanic:
	default:
	}
	p.cancel()
	fmt.Printf("Caught goroutine panic:\n\n%s\n\nRestoring terminal...\n\n", r)
	debug.PrintStack()
}

// ReleaseTerminal restores the original terminal state and cancels the input
// reader. You can return control to the Program with RestoreTerminal.
func (p *Program) ReleaseTerminal() error {
	atomic.StoreUint32(&p.ignoreSignals, 1)
	if p.cancelReader != nil {
		p.cancelReader.Cancel()
	}

	p.waitForReadLoop()

	if p.renderer != nil {
		p.renderer.Stop()
		p.altScreenWasActive = p.renderer.AltScreen()
		p.bpWasActive = p.renderer.BracketedPasteActive()
		p.reportFocus = p.renderer.ReportFocus()
	}

	return p.restoreTerminalState()
}

// RestoreTerminal reinitializes the Program's input reader, restores the
// terminal to the former state when the program was running, and repaints.
// Use it to reinitialize a Program after running ReleaseTerminal.
func (p *Program) RestoreTerminal() error {
	atomic.StoreUint32(&p.ignoreSignals, 0)

	if err := p.initTerminal(); err != nil {
		return err
	}
	if err := p.initCancelReader(false); err != nil {
		return err
	}
	if p.altScreenWasActive {
		p.renderer.EnterAltScreen()
	} else {
		// entering alt screen already causes a repaint.
		go p.Send(repaintMsg{})
	}
	if p.renderer != nil {
		p.renderer.Start()
	}
	if p.bpWasActive {
		p.renderer.EnableBracketedPaste()
	}
	if p.reportFocus {
		p.renderer.EnableReportFocus()
	}

	// If the output is a terminal, it may have been resized while another
	// process was at the foreground, in which case we may not have received
	// SIGWINCH. Detect any size change now and propagate the new size as
	// needed.
	go p.checkResize()

	return nil
}

// Println prints above the Program. This output is unmanaged by the program
// and will persist across renders by the Program.
//
// If the altscreen is active no output will be printed.
func (p *Program) Println(args ...interface{}) {
	p.msgs <- printLineMessage{
		messageBody: fmt.Sprint(args...),
	}
}

// Printf prints above the Program. It takes a format template followed by
// values similar to fmt.Printf. This output is unmanaged by the program and
// will persist across renders by the Program.
//
// Unlike fmt.Printf (but similar to log.Printf) the message will be print on
// its own line.
//
// If the altscreen is active no output will be printed.
func (p *Program) Printf(template string, args ...interface{}) {
	p.msgs <- printLineMessage{
		messageBody: fmt.Sprintf(template, args...),
	}
}
