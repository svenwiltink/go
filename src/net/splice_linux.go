// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import (
	"internal/poll"
	"io"
	"os"
)

// splice transfers data from r to c using the splice system call to minimize
// copies from and to userspace. c must be a TCP connection. Currently, splice
// is only enabled if r is a TCP or a stream-oriented Unix connection.
//
// If splice returns handled == false, it has performed no work.
func splice(c *netFD, r io.Reader) (written int64, err error, handled bool) {
	var remain int64 = 1 << 62 // by default, copy until EOF
	lr, ok := r.(*io.LimitedReader)
	if ok {
		remain, r = lr.N, lr.R
		if remain <= 0 {
			return 0, nil, true
		}
	}

	var s *netFD
	if tc, ok := r.(*TCPConn); ok {
		s = tc.fd
	} else if uc, ok := r.(*UnixConn); ok {
		if uc.fd.net != "unix" {
			return 0, nil, false
		}
		s = uc.fd
	} else {
		return 0, nil, false
	}

	written, handled, sc, err := poll.Splice(&c.pfd, &s.pfd, remain)
	if lr != nil {
		lr.N -= written
	}
	return written, wrapSyscallError(sc, err), handled
}

// splice transfers data from c to w using the splice system call to minimize
// copies from and to userspace. c must be a TCP connection. Currently, splice
// is only enabled if w is an *os.File
//
// If splice returns handled == false, it has performed no work.
func spliceWriteN(c *netFD, remain int64, w io.Writer) (written int64, err error, handled bool) {
	var fd *poll.FD
	if f, ok := w.(*os.File); ok {
		fd = &poll.FD{
			Sysfd:         int(f.Fd()),
			IsStream:      true,
			ZeroReadIsEOF: true,
		}
	} else {
		return 0, nil, false
	}

	written, handled, sc, err := poll.Splice(fd, &c.pfd, remain)
	return written, wrapSyscallError(sc, err), handled
}
