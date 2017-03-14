/*
 *
 * Copyright 2014, Google Inc.
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are
 * met:
 *
 *     * Redistributions of source code must retain the above copyright
 * notice, this list of conditions and the following disclaimer.
 *     * Redistributions in binary form must reproduce the above
 * copyright notice, this list of conditions and the following disclaimer
 * in the documentation and/or other materials provided with the
 * distribution.
 *     * Neither the name of Google Inc. nor the names of its
 * contributors may be used to endorse or promote products derived from
 * this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 *
 */

/*
 * This log filtering is lifted from grpc's own end-to-end tests:
 * 	https://github.com/grpc/grpc-go/blob/master/test/end2end_test.go
 *
 * See reproduced license above.
 */

package acceptance

import (
	"bytes"
	"flag"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/grpclog"
)

var verboseLogs = flag.Bool("verbose_logs", false, "show all grpclog output, without filtering")
var testLogOutput = &lockingWriter{w: os.Stderr}

func init() {
	grpclog.SetLogger(log.New(testLogOutput, "", log.LstdFlags))
}

type lockingWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (lw *lockingWriter) Write(p []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.w.Write(p)
}

func (lw *lockingWriter) setWriter(w io.Writer) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	lw.w = w
}

// awaitNewConnLogOutput waits for any of grpc.NewConn's goroutines to
// terminate, if they're still running. It spams logs with this
// message.  We wait for it so our log filter is still
// active. Otherwise the "defer restore()" at the top of various test
// functions restores our log filter and then the goroutine spams.
func awaitNewConnLogOutput() {
	awaitLogOutput(50*time.Millisecond, "grpc: the client connection is closing; please retry")
}

func awaitLogOutput(maxWait time.Duration, phrase string) {
	pb := []byte(phrase)

	timer := time.NewTimer(maxWait)
	defer timer.Stop()
	wakeup := make(chan bool, 1)
	for {
		if logOutputHasContents(pb, wakeup) {
			return
		}
		select {
		case <-timer.C:
			// Too slow. Oh well.
			return
		case <-wakeup:
		}
	}
}

func logOutputHasContents(v []byte, wakeup chan<- bool) bool {
	testLogOutput.mu.Lock()
	defer testLogOutput.mu.Unlock()
	fw, ok := testLogOutput.w.(*filterWriter)
	if !ok {
		return false
	}
	fw.mu.Lock()
	defer fw.mu.Unlock()
	if bytes.Contains(fw.buf.Bytes(), v) {
		return true
	}
	fw.wakeup = wakeup
	return false
}

func noop() {}

// declareLogNoise declares that t is expected to emit the following noisy phrases,
// even on success. Those phrases will be filtered from grpclog output
// and only be shown if *verbose_logs or t ends up failing.
// The returned restore function should be called with defer to be run
// before the test ends.
func declareLogNoise(t *testing.T, phrases ...string) (restore func()) {
	if *verboseLogs {
		return noop
	}
	fw := &filterWriter{dst: os.Stderr, filter: phrases}
	testLogOutput.setWriter(fw)
	return func() {
		if t.Failed() {
			fw.mu.Lock()
			defer fw.mu.Unlock()
			if fw.buf.Len() > 0 {
				t.Logf("Complete log output:\n%s", fw.buf.Bytes())
			}
		}
		testLogOutput.setWriter(os.Stderr)
	}
}

type filterWriter struct {
	dst    io.Writer
	filter []string

	mu     sync.Mutex
	buf    bytes.Buffer
	wakeup chan<- bool // if non-nil, gets true on write
}

func (fw *filterWriter) Write(p []byte) (n int, err error) {
	fw.mu.Lock()
	fw.buf.Write(p)
	if fw.wakeup != nil {
		select {
		case fw.wakeup <- true:
		default:
		}
	}
	fw.mu.Unlock()

	ps := string(p)
	for _, f := range fw.filter {
		if strings.Contains(ps, f) {
			return len(p), nil
		}
	}
	return fw.dst.Write(p)
}
