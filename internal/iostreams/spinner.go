package iostreams

import (
	"fmt"
	"sync"
	"time"
)

// spinnerFrames is a Braille spinner — compact and widely supported.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner is a single-line progress indicator that animates on stderr. It is
// chrome, never data: when progress is disabled (piped stderr, --quiet,
// --hide-spinner) every method is a no-op, so it is always safe to use.
type Spinner struct {
	io       *IOStreams
	interval time.Duration

	mu      sync.Mutex
	msg     string
	active  bool
	stop    chan struct{}
	stopped chan struct{}
}

// NewSpinner creates a spinner with an initial message. Call Start to animate
// and Stop (typically deferred) to clear it.
func (s *IOStreams) NewSpinner(msg string) *Spinner {
	return &Spinner{io: s, msg: msg, interval: 100 * time.Millisecond}
}

// enabled reports whether the spinner should render at all.
func (sp *Spinner) enabled() bool { return sp.io.progressEnabled && sp.io.stderrTTY }

// Start begins animating. It is a no-op if progress is disabled or already
// running.
func (sp *Spinner) Start() {
	if !sp.enabled() {
		return
	}
	sp.mu.Lock()
	if sp.active {
		sp.mu.Unlock()
		return
	}
	sp.active = true
	sp.stop = make(chan struct{})
	sp.stopped = make(chan struct{})
	msg := sp.msg
	sp.mu.Unlock()

	// Paint the first frame immediately so the spinner is visible at once.
	sp.paint(spinnerFrames[0], msg)

	go sp.run()
}

func (sp *Spinner) run() {
	defer close(sp.stopped)
	ticker := time.NewTicker(sp.interval)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-sp.stop:
			return
		case <-ticker.C:
			i = (i + 1) % len(spinnerFrames)
			sp.mu.Lock()
			msg := sp.msg
			sp.mu.Unlock()
			sp.paint(spinnerFrames[i], msg)
		}
	}
}

func (sp *Spinner) paint(frame, msg string) {
	fmt.Fprintf(sp.io.ErrOut, "\r%s %s", sp.io.colorize(codeCyan, frame), msg)
}

// Update changes the message shown next to the spinner.
func (sp *Spinner) Update(msg string) {
	sp.mu.Lock()
	sp.msg = msg
	sp.mu.Unlock()
}

// Stop halts the animation and clears the spinner line. Safe to call multiple
// times and when the spinner never started.
func (sp *Spinner) Stop() {
	sp.mu.Lock()
	if !sp.active {
		sp.mu.Unlock()
		return
	}
	sp.active = false
	close(sp.stop)
	stopped := sp.stopped
	sp.mu.Unlock()

	<-stopped
	// Clear the line: carriage return, blanks, carriage return.
	fmt.Fprintf(sp.io.ErrOut, "\r%s\r", spaces(len(sp.msg)+2))
}

func spaces(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
