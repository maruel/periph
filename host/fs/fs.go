// Copyright 2017 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package fs provides access to the file system on the host.
//
// It exposes ioctl syscall and epoll in an OS agnostic way and permits
// completely disabling file access to lock down unit tests.
package fs

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"
)

// Ioctler is a file handle that supports ioctl calls.
type Ioctler interface {
	// Ioctl sends a linux ioctl on the file handle.
	//
	// op is effectively an uint32. op is expected to be encoded in the format on
	// x64. ARM happens to share the same format.
	Ioctl(op uint, data uintptr) error
}

// Open opens a file.
//
// Returns an error if Inhibit() was called.
func Open(path string, flag int) (*File, error) {
	mu.Lock()
	if inhibited {
		mu.Unlock()
		return nil, errors.New("file I/O is inhibited")
	}
	used = true
	mu.Unlock()

	f, err := os.OpenFile(path, flag, 0600)
	if err != nil {
		return nil, err
	}
	return &File{f}, nil
}

// Inhibit inhibits any future file I/O. It panics if any file was opened up to
// now.
//
// It should only be called in unit tests.
func Inhibit() {
	mu.Lock()
	inhibited = true
	if used {
		panic("calling Inhibit() while files were already opened")
	}
	mu.Unlock()
}

// File is a superset of os.File.
type File struct {
	*os.File
}

// Ioctl sends an ioctl to the file handle.
func (f *File) Ioctl(op uint, data uintptr) error {
	return ioctl(f.Fd(), op, data)
}

// Event is a file system event.
//
// Deprecated: Use ListenEdges instead.
type Event struct {
	event
}

// MakeEvent initializes an epoll *edge* triggered event on linux.
//
// An edge triggered event is basically an "auto-reset" event, where waiting on
// the edge resets it. A level triggered event requires manual resetting; this
// could be done via a Read() call but there's no need to require the user to
// call Read(). This is particularly useless in the case of gpio.RisingEdge and
// gpio.FallingEdge.
//
// As per the official doc, edge triggers is still remembered even when no
// epoll_wait() call is running, so no edge is missed. Two edges will be
// coallesced into one if the user mode process can't keep up. There's no
// accumulation of edges.
//
// Deprecated: to be removed in v4.0.0.
func (e *Event) MakeEvent(fd uintptr) error {
	return e.event.makeEvent(fd)
}

// Wait waits for an event or the specified amount of time.
//
// Returns the unfiltered epoll events value.
//
// Deprecated: Use WaitCtx instead.
func (e *Event) Wait(timeoutms int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutms)*time.Millisecond)
	t := e.event.wait(ctx)
	cancel()
	if t.IsZero() {
		// 0 means timeout.
		return 0, nil
	}
	return 1, nil
}

// WaitCtx waits for an event or until the context is Done.
//
// Return a zero Time if no event was detected.
func (e *Event) WaitCtx(ctx context.Context) time.Time {
	return e.event.wait(ctx)
}

// Peek returns if a event was already pending, without waiting.
func (e *Event) Peek() time.Time {
	return e.event.peek()
}

// ClearAccumulated clears any accumulated edge.
func (e *Event) ClearAccumulated() {
	e.event.clearAccumulated()
}

//

// ListenEdges listens for epoll edges on an OS file descriptor.
func ListenEdges(ctx context.Context, f *os.File, c chan<- time.Time) error {
	return events.listen(ctx, f.Fd(), c)
}

//

var (
	mu        sync.Mutex
	inhibited bool
	used      bool
)
