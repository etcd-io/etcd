package pb

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/fatih/color"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"

	"github.com/cheggaaa/pb/v3/termutil"
)

// Version of ProgressBar library
const Version = "3.0.8"

type key int

const (
	// Bytes means we're working with byte sizes. Numbers will print as Kb, Mb, etc
	// bar.Set(pb.Bytes, true)
	Bytes key = 1 << iota

	// Use SI bytes prefix names (kB, MB, etc) instead of IEC prefix names (KiB, MiB, etc)
	SIBytesPrefix

	// Terminal means we're will print to terminal and can use ascii sequences
	// Also we're will try to use terminal width
	Terminal

	// Static means progress bar will not update automaticly
	Static

	// ReturnSymbol - by default in terminal mode it's '\r'
	ReturnSymbol

	// Color by default is true when output is tty, but you can set to false for disabling colors
	Color

	// Hide the progress bar when finished, rather than leaving it up. By default it's false.
	CleanOnFinish

	// Round elapsed time to this precision. Defaults to time.Second.
	TimeRound
)

const (
	defaultBarWidth    = 100
	defaultRefreshRate = time.Millisecond * 200
)

// New creates new ProgressBar object
func New(total int) *ProgressBar {
	return New64(int64(total))
}

// New64 creates new ProgressBar object using int64 as total
func New64(total int64) *ProgressBar {
	pb := new(ProgressBar)
	return pb.SetTotal(total)
}

// StartNew starts new ProgressBar with Default template
func StartNew(total int) *ProgressBar {
	return New(total).Start()
}

// Start64 starts new ProgressBar with Default template. Using int64 as total.
func Start64(total int64) *ProgressBar {
	return New64(total).Start()
}

var (
	terminalWidth    = termutil.TerminalWidth
	isTerminal       = isatty.IsTerminal
	isCygwinTerminal = isatty.IsCygwinTerminal
)

// ProgressBar is the main object of bar
type ProgressBar struct {
	current, total int64
	width          int
	maxWidth       int
	mu             sync.RWMutex
	rm             sync.Mutex
	vars           map[interface{}]interface{}
	elements       map[string]Element
	output         io.Writer
	coutput        io.Writer
	nocoutput      io.Writer
	startTime      time.Time
	refreshRate    time.Duration
	tmpl           *template.Template
	state          *State
	buf            *bytes.Buffer
	ticker         *time.Ticker
	finish         chan struct{}
	finished       bool
	configured     bool
	err            error
}

func (pb *ProgressBar) configure() {
	if pb.configured {
		return
	}
	pb.configured = true

	if pb.vars == nil {
		pb.vars = make(map[interface{}]interface{})
	}
	if pb.output == nil {
		pb.output = os.Stderr
	}

	if pb.tmpl == nil {
		pb.tmpl, pb.err = getTemplate(string(Default))
		if pb.err != nil {
			return
		}
	}
	if pb.vars[Terminal] == nil {
		if f, ok := pb.output.(*os.File); ok {
			if isTerminal(f.Fd()) || isCygwinTerminal(f.Fd()) {
				pb.vars[Terminal] = true
			}
		}
	}
	if pb.vars[ReturnSymbol] == nil {
		if tm, ok := pb.vars[Terminal].(bool); ok && tm {
			pb.vars[ReturnSymbol] = "\r"
		}
	}
	if pb.vars[Color] == nil {
		if tm, ok := pb.vars[Terminal].(bool); ok && tm {
			pb.vars[Color] = true
		}
	}
	if pb.refreshRate == 0 {
		pb.refreshRate = defaultRefreshRate
	}
	if pb.vars[CleanOnFinish] == nil {
		pb.vars[CleanOnFinish] = false
	}
	if f, ok := pb.output.(*os.File); ok {
		pb.coutput = colorable.NewColorable(f)
	} else {
		pb.coutput = pb.output
	}
	pb.nocoutput = colorable.NewNonColorable(pb.output)
}

// Start starts the bar
func (pb *ProgressBar) Start() *ProgressBar {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	if pb.finish != nil {
		return pb
	}
	pb.configure()
	pb.finished = false
	pb.state = nil
	pb.startTime = time.Now()
	if st, ok := pb.vars[Static].(bool); ok && st {
		return pb
	}
	pb.finish = make(chan struct{})
	pb.ticker = time.NewTicker(pb.refreshRate)
	go pb.writer(pb.finish)
	return pb
}

func (pb *ProgressBar) writer(finish chan struct{}) {
	for {
		select {
		case <-pb.ticker.C:
			pb.write(false)
		case <-finish:
			pb.ticker.Stop()
			pb.write(true)
			finish <- struct{}{}
			return
		}
	}
}

// Write performs write to the output
func (pb *ProgressBar) Write() *ProgressBar {
	pb.mu.RLock()
	finished := pb.finished
	pb.mu.RUnlock()
	pb.write(finished)
	return pb
}

func (pb *ProgressBar) write(finish bool) {
	result, width := pb.render()
	if pb.Err() != nil {
		return
	}
	if pb.GetBool(Terminal) {
		if r := (width - CellCount(result)); r > 0 {
			result += strings.Repeat(" ", r)
		}
	}
	if ret, ok := pb.Get(ReturnSymbol).(string); ok {
		result = ret + result
		if finish && ret == "\r" {
			if pb.GetBool(CleanOnFinish) {
				// "Wipe out" progress bar by overwriting one line with blanks
				result = "\r" + color.New(color.Reset).Sprint(strings.Repeat(" ", width)) + "\r"
			} else {
				result += "\n"
			}
		}
	}
	if pb.GetBool(Color) {
		pb.coutput.Write([]byte(result))
	} else {
		pb.nocoutput.Write([]byte(result))
	}
}

// Total return current total bar value
func (pb *ProgressBar) Total() int64 {
	return atomic.LoadInt64(&pb.total)
}

// SetTotal sets the total bar value
func (pb *ProgressBar) SetTotal(value int64) *ProgressBar {
	atomic.StoreInt64(&pb.total, value)
	return pb
}

// AddTotal adds to the total bar value
func (pb *ProgressBar) AddTotal(value int64) *ProgressBar {
	atomic.AddInt64(&pb.total, value)
	return pb
}

// SetCurrent sets the current bar value
func (pb *ProgressBar) SetCurrent(value int64) *ProgressBar {
	atomic.StoreInt64(&pb.current, value)
	return pb
}

// Current return current bar value
func (pb *ProgressBar) Current() int64 {
	return atomic.LoadInt64(&pb.current)
}

// Add adding given int64 value to bar value
func (pb *ProgressBar) Add64(value int64) *ProgressBar {
	atomic.AddInt64(&pb.current, value)
	return pb
}

// Add adding given int value to bar value
func (pb *ProgressBar) Add(value int) *ProgressBar {
	return pb.Add64(int64(value))
}

// Increment atomically increments the progress
func (pb *ProgressBar) Increment() *ProgressBar {
	return pb.Add64(1)
}

// Set sets any value by any key
func (pb *ProgressBar) Set(key, value interface{}) *ProgressBar {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	if pb.vars == nil {
		pb.vars = make(map[interface{}]interface{})
	}
	pb.vars[key] = value
	return pb
}

// Get return value by key
func (pb *ProgressBar) Get(key interface{}) interface{} {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	if pb.vars == nil {
		return nil
	}
	return pb.vars[key]
}

// GetBool return value by key and try to convert there to boolean
// If value doesn't set or not boolean - return false
func (pb *ProgressBar) GetBool(key interface{}) bool {
	if v, ok := pb.Get(key).(bool); ok {
		return v
	}
	return false
}

// SetWidth sets the bar width
// When given value <= 0 would be using the terminal width (if possible) or default value.
func (pb *ProgressBar) SetWidth(width int) *ProgressBar {
	pb.mu.Lock()
	pb.width = width
	pb.mu.Unlock()
	return pb
}

// SetMaxWidth sets the bar maximum width
// When given value <= 0 would be using the terminal width (if possible) or default value.
func (pb *ProgressBar) SetMaxWidth(maxWidth int) *ProgressBar {
	pb.mu.Lock()
	pb.maxWidth = maxWidth
	pb.mu.Unlock()
	return pb
}

// Width return the bar width
// It's current terminal width or settled over 'SetWidth' value.
func (pb *ProgressBar) Width() (width int) {
	defer func() {
		if r := recover(); r != nil {
			width = defaultBarWidth
		}
	}()
	pb.mu.RLock()
	width = pb.width
	maxWidth := pb.maxWidth
	pb.mu.RUnlock()
	if width <= 0 {
		var err error
		if width, err = terminalWidth(); err != nil {
			return defaultBarWidth
		}
	}
	if maxWidth > 0 && width > maxWidth {
		width = maxWidth
	}
	return
}

func (pb *ProgressBar) SetRefreshRate(dur time.Duration) *ProgressBar {
	pb.mu.Lock()
	if dur > 0 {
		pb.refreshRate = dur
	}
	pb.mu.Unlock()
	return pb
}

// SetWriter sets the io.Writer. Bar will write in this writer
// By default this is os.Stderr
func (pb *ProgressBar) SetWriter(w io.Writer) *ProgressBar {
	pb.mu.Lock()
	pb.output = w
	pb.configured = false
	pb.configure()
	pb.mu.Unlock()
	return pb
}

// StartTime return the time when bar started
func (pb *ProgressBar) StartTime() time.Time {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return pb.startTime
}

// Format convert int64 to string according to the current settings
func (pb *ProgressBar) Format(v int64) string {
	if pb.GetBool(Bytes) {
		return formatBytes(v, pb.GetBool(SIBytesPrefix))
	}
	return strconv.FormatInt(v, 10)
}

// Finish stops the bar
func (pb *ProgressBar) Finish() *ProgressBar {
	pb.mu.Lock()
	if pb.finished {
		pb.mu.Unlock()
		return pb
	}
	finishChan := pb.finish
	pb.finished = true
	pb.mu.Unlock()
	if finishChan != nil {
		finishChan <- struct{}{}
		<-finishChan
		pb.mu.Lock()
		pb.finish = nil
		pb.mu.Unlock()
	}
	return pb
}

// IsStarted indicates progress bar state
func (pb *ProgressBar) IsStarted() bool {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return pb.finish != nil
}

// IsFinished indicates progress bar is finished
func (pb *ProgressBar) IsFinished() bool {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return pb.finished
}

// SetTemplateString sets ProgressBar tempate string and parse it
func (pb *ProgressBar) SetTemplateString(tmpl string) *ProgressBar {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.tmpl, pb.err = getTemplate(tmpl)
	return pb
}

// SetTemplateString sets ProgressBarTempate and parse it
func (pb *ProgressBar) SetTemplate(tmpl ProgressBarTemplate) *ProgressBar {
	return pb.SetTemplateString(string(tmpl))
}

// NewProxyReader creates a wrapper for given reader, but with progress handle
// Takes io.Reader or io.ReadCloser
// Also, it automatically switches progress bar to handle units as bytes
func (pb *ProgressBar) NewProxyReader(r io.Reader) *Reader {
	pb.Set(Bytes, true)
	return &Reader{r, pb}
}

// NewProxyWriter creates a wrapper for given writer, but with progress handle
// Takes io.Writer or io.WriteCloser
// Also, it automatically switches progress bar to handle units as bytes
func (pb *ProgressBar) NewProxyWriter(r io.Writer) *Writer {
	pb.Set(Bytes, true)
	return &Writer{r, pb}
}

func (pb *ProgressBar) render() (result string, width int) {
	defer func() {
		if r := recover(); r != nil {
			pb.SetErr(fmt.Errorf("render panic: %v", r))
		}
	}()
	pb.rm.Lock()
	defer pb.rm.Unlock()
	pb.mu.Lock()
	pb.configure()
	if pb.state == nil {
		pb.state = &State{ProgressBar: pb}
		pb.buf = bytes.NewBuffer(nil)
	}
	if pb.startTime.IsZero() {
		pb.startTime = time.Now()
	}
	pb.state.id++
	pb.state.finished = pb.finished
	pb.state.time = time.Now()
	pb.mu.Unlock()

	pb.state.width = pb.Width()
	width = pb.state.width
	pb.state.total = pb.Total()
	pb.state.current = pb.Current()
	pb.buf.Reset()

	if e := pb.tmpl.Execute(pb.buf, pb.state); e != nil {
		pb.SetErr(e)
		return "", 0
	}

	result = pb.buf.String()

	aec := len(pb.state.recalc)
	if aec == 0 {
		// no adaptive elements
		return
	}

	staticWidth := CellCount(result) - (aec * adElPlaceholderLen)

	if pb.state.Width()-staticWidth <= 0 {
		result = strings.Replace(result, adElPlaceholder, "", -1)
		result = StripString(result, pb.state.Width())
	} else {
		pb.state.adaptiveElWidth = (width - staticWidth) / aec
		for _, el := range pb.state.recalc {
			result = strings.Replace(result, adElPlaceholder, el.ProgressElement(pb.state), 1)
		}
	}
	pb.state.recalc = pb.state.recalc[:0]
	return
}

// SetErr sets error to the ProgressBar
// Error will be available over Err()
func (pb *ProgressBar) SetErr(err error) *ProgressBar {
	pb.mu.Lock()
	pb.err = err
	pb.mu.Unlock()
	return pb
}

// Err return possible error
// When all ok - will be nil
// May contain template.Execute errors
func (pb *ProgressBar) Err() error {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return pb.err
}

// String return currrent string representation of ProgressBar
func (pb *ProgressBar) String() string {
	res, _ := pb.render()
	return res
}

// ProgressElement implements Element interface
func (pb *ProgressBar) ProgressElement(s *State, args ...string) string {
	if s.IsAdaptiveWidth() {
		pb.SetWidth(s.AdaptiveElWidth())
	}
	return pb.String()
}

// State represents the current state of bar
// Need for bar elements
type State struct {
	*ProgressBar

	id                     uint64
	total, current         int64
	width, adaptiveElWidth int
	finished, adaptive     bool
	time                   time.Time

	recalc []Element
}

// Id it's the current state identifier
// - incremental
// - starts with 1
// - resets after finish/start
func (s *State) Id() uint64 {
	return s.id
}

// Total it's bar int64 total
func (s *State) Total() int64 {
	return s.total
}

// Value it's current value
func (s *State) Value() int64 {
	return s.current
}

// Width of bar
func (s *State) Width() int {
	return s.width
}

// AdaptiveElWidth - adaptive elements must return string with given cell count (when AdaptiveElWidth > 0)
func (s *State) AdaptiveElWidth() int {
	return s.adaptiveElWidth
}

// IsAdaptiveWidth returns true when element must be shown as adaptive
func (s *State) IsAdaptiveWidth() bool {
	return s.adaptive
}

// IsFinished return true when bar is finished
func (s *State) IsFinished() bool {
	return s.finished
}

// IsFirst return true only in first render
func (s *State) IsFirst() bool {
	return s.id == 1
}

// Time when state was created
func (s *State) Time() time.Time {
	return s.time
}
