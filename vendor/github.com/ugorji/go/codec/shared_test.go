// Copyright (c) 2012-2015 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

// This file sets up the variables used, including testInitFns.
// Each file should add initialization that should be performed
// after flags are parsed.
//
// init is a multi-step process:
//   - setup vars (handled by init functions in each file)
//   - parse flags
//   - setup derived vars (handled by pre-init registered functions - registered in init function)
//   - post init (handled by post-init registered functions - registered in init function)
// This way, no one has to manage carefully control the initialization
// using file names, etc.
//
// Tests which require external dependencies need the -tag=x parameter.
// They should be run as:
//    go test -tags=x -run=. <other parameters ...>
// Benchmarks should also take this parameter, to include the sereal, xdr, etc.
// To run against codecgen, etc, make sure you pass extra parameters.
// Example usage:
//    go test "-tags=x codecgen" -bench=. <other parameters ...>
//
// To fully test everything:
//    go test -tags=x -benchtime=100ms -tv -bg -bi  -brw -bu -v -run=. -bench=.

// Handling flags
// codec_test.go will define a set of global flags for testing, including:
//   - Use Reset
//   - Use IO reader/writer (vs direct bytes)
//   - Set Canonical
//   - Set InternStrings
//   - Use Symbols
//
// This way, we can test them all by running same set of tests with a different
// set of flags.
//
// Following this, all the benchmarks will utilize flags set by codec_test.go
// and will not redefine these "global" flags.

import (
	"bytes"
	"flag"
	"io"
	"sync"
)

// DO NOT REMOVE - replacement line for go-codec-bench import declaration tag //

type testHED struct {
	H Handle
	E *Encoder
	D *Decoder
}

type ioReaderWrapper struct {
	r io.Reader
}

func (x ioReaderWrapper) Read(p []byte) (n int, err error) {
	return x.r.Read(p)
}

type ioWriterWrapper struct {
	w io.Writer
}

func (x ioWriterWrapper) Write(p []byte) (n int, err error) {
	return x.w.Write(p)
}

var (
	// testNoopH    = NoopHandle(8)
	testMsgpackH = &MsgpackHandle{}
	testBincH    = &BincHandle{}
	testSimpleH  = &SimpleHandle{}
	testCborH    = &CborHandle{}
	testJsonH    = &JsonHandle{}

	testHandles     []Handle
	testPreInitFns  []func()
	testPostInitFns []func()

	testOnce sync.Once

	testHEDs []testHED
)

// flag variables used by tests (and bench)
var (
	testDepth int

	testVerbose       bool
	testInitDebug     bool
	testStructToArray bool
	testCanonical     bool
	testUseReset      bool
	testSkipIntf      bool
	testInternStr     bool
	testUseMust       bool
	testCheckCircRef  bool

	testUseIoEncDec  int
	testUseIoWrapper bool

	testMaxInitLen int

	testNumRepeatString int
)

// variables that are not flags, but which can configure the handles
var (
	testEncodeOptions EncodeOptions
	testDecodeOptions DecodeOptions
)

// flag variables used by bench
var (
	benchDoInitBench      bool
	benchVerify           bool
	benchUnscientificRes  bool = false
	benchMapStringKeyOnly bool
	//depth of 0 maps to ~400bytes json-encoded string, 1 maps to ~1400 bytes, etc
	//For depth>1, we likely trigger stack growth for encoders, making benchmarking unreliable.
	benchDepth     int
	benchInitDebug bool
)

func init() {
	testHEDs = make([]testHED, 0, 32)
	testHandles = append(testHandles,
		// testNoopH,
		testMsgpackH, testBincH, testSimpleH,
		testCborH, testJsonH)
	testInitFlags()
	benchInitFlags()
}

func testInitFlags() {
	// delete(testDecOpts.ExtFuncs, timeTyp)
	flag.IntVar(&testDepth, "tsd", 0, "Test Struc Depth")
	flag.BoolVar(&testVerbose, "tv", false, "Test Verbose")
	flag.BoolVar(&testInitDebug, "tg", false, "Test Init Debug")
	flag.IntVar(&testUseIoEncDec, "ti", -1, "Use IO Reader/Writer for Marshal/Unmarshal ie >= 0")
	flag.BoolVar(&testUseIoWrapper, "tiw", false, "Wrap the IO Reader/Writer with a base pass-through reader/writer")
	flag.BoolVar(&testStructToArray, "ts", false, "Set StructToArray option")
	flag.BoolVar(&testCanonical, "tc", false, "Set Canonical option")
	flag.BoolVar(&testInternStr, "te", false, "Set InternStr option")
	flag.BoolVar(&testSkipIntf, "tf", false, "Skip Interfaces")
	flag.BoolVar(&testUseReset, "tr", false, "Use Reset")
	flag.IntVar(&testNumRepeatString, "trs", 8, "Create string variables by repeating a string N times")
	flag.IntVar(&testMaxInitLen, "tx", 0, "Max Init Len")
	flag.BoolVar(&testUseMust, "tm", true, "Use Must(En|De)code")
	flag.BoolVar(&testCheckCircRef, "tl", false, "Use Check Circular Ref")
}

func benchInitFlags() {
	flag.BoolVar(&benchMapStringKeyOnly, "bs", false, "Bench use maps with string keys only")
	flag.BoolVar(&benchInitDebug, "bg", false, "Bench Debug")
	flag.IntVar(&benchDepth, "bd", 1, "Bench Depth")
	flag.BoolVar(&benchDoInitBench, "bi", false, "Run Bench Init")
	flag.BoolVar(&benchVerify, "bv", false, "Verify Decoded Value during Benchmark")
	flag.BoolVar(&benchUnscientificRes, "bu", false, "Show Unscientific Results during Benchmark")
}

func testHEDGet(h Handle) *testHED {
	for i := range testHEDs {
		v := &testHEDs[i]
		if v.H == h {
			return v
		}
	}
	testHEDs = append(testHEDs, testHED{h, NewEncoder(nil, h), NewDecoder(nil, h)})
	return &testHEDs[len(testHEDs)-1]
}

func testReinit() {
	testOnce = sync.Once{}
	testHEDs = nil
}

func testInitAll() {
	// only parse it once.
	if !flag.Parsed() {
		flag.Parse()
	}
	for _, f := range testPreInitFns {
		f()
	}
	for _, f := range testPostInitFns {
		f()
	}
}

func testCodecEncode(ts interface{}, bsIn []byte,
	fn func([]byte) *bytes.Buffer, h Handle) (bs []byte, err error) {
	// bs = make([]byte, 0, approxSize)
	var e *Encoder
	var buf *bytes.Buffer
	if testUseReset {
		e = testHEDGet(h).E
	} else {
		e = NewEncoder(nil, h)
	}
	bh := BasicHandleDoNotUse(h)
	var oldWriteBufferSize int
	if testUseIoEncDec >= 0 {
		buf = fn(bsIn)
		// set the encode options for using a buffer
		oldWriteBufferSize = bh.WriterBufferSize
		bh.WriterBufferSize = testUseIoEncDec
		if testUseIoWrapper {
			e.Reset(ioWriterWrapper{buf})
		} else {
			e.Reset(buf)
		}
	} else {
		bs = bsIn
		e.ResetBytes(&bs)
	}
	if testUseMust {
		e.MustEncode(ts)
	} else {
		err = e.Encode(ts)
	}
	if testUseIoEncDec >= 0 {
		bs = buf.Bytes()
		bh.WriterBufferSize = oldWriteBufferSize
	}
	return
}

func testCodecDecode(bs []byte, ts interface{}, h Handle) (err error) {
	var d *Decoder
	// var buf *bytes.Reader
	if testUseReset {
		d = testHEDGet(h).D
	} else {
		d = NewDecoder(nil, h)
	}
	bh := BasicHandleDoNotUse(h)
	var oldReadBufferSize int
	if testUseIoEncDec >= 0 {
		buf := bytes.NewReader(bs)
		oldReadBufferSize = bh.ReaderBufferSize
		bh.ReaderBufferSize = testUseIoEncDec
		if testUseIoWrapper {
			d.Reset(ioReaderWrapper{buf})
		} else {
			d.Reset(buf)
		}
	} else {
		d.ResetBytes(bs)
	}
	if testUseMust {
		d.MustDecode(ts)
	} else {
		err = d.Decode(ts)
	}
	if testUseIoEncDec >= 0 {
		bh.ReaderBufferSize = oldReadBufferSize
	}
	return
}

// ----- functions below are used only by benchmarks alone

func fnBenchmarkByteBuf(bsIn []byte) (buf *bytes.Buffer) {
	// var buf bytes.Buffer
	// buf.Grow(approxSize)
	buf = bytes.NewBuffer(bsIn)
	buf.Truncate(0)
	return
}

func benchFnCodecEncode(ts interface{}, bsIn []byte, h Handle) (bs []byte, err error) {
	return testCodecEncode(ts, bsIn, fnBenchmarkByteBuf, h)
}

func benchFnCodecDecode(bs []byte, ts interface{}, h Handle) (err error) {
	return testCodecDecode(bs, ts, h)
}
