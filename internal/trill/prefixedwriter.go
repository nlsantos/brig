/*
   trill: a lightweight wrapper for Podman/Docker REST API calls
   Copyright (C) 2025  Neil Santos

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.
*/

// Package trill houses a thin wrapper for communicating with podman
// and Docker via their REST API.
package trill

import (
	"bytes"
	"fmt"
	"io"

	"github.com/fatih/color"
)

// NewFrefixedPrintfError returns a function that can be used in place
// of fmt.Printf when outputting errors; every invocation prints out a
// standardized prefix before the rest of its arguments.
func NewPrefixedPrintffError(action string) func(format string, a ...any) (n int, err error) {
	return func(format string, a ...any) (n int, err error) {
		c_action := color.New(color.FgGreen).SprintFunc()
		c_error := color.New(color.BgHiRed, color.FgBlack, color.Bold).SprintFunc()
		params := []any{c_action(" " + action + " "), c_error(" ERROR ")}
		params = append(params, a...)
		return fmt.Fprintf(color.Output, "%s %s "+format, params...)
	}
}

// NewFrefixedPrintf returns a function that can be used in place of
// fmt.Printf; every invocation prints out a standardized prefix
// before the rest of its arguments.
func NewPrefixedPrintff(action string, context string) func(format string, a ...any) (n int, err error) {
	return func(format string, a ...any) (n int, err error) {
		c_action := color.New(color.BgHiGreen, color.FgBlack).SprintFunc()
		c_context := color.New(color.FgHiWhite).SprintFunc()
		params := []any{c_action(" " + action + " "), c_context(context)}
		params = append(params, a...)
		return fmt.Fprintf(color.Output, "%s %s "+format, params...)
	}
}

// StreamWriter is a thin custom wrapper for outputting streaming
// messages with a prefix at the beginning of each line.
type StreamWriter struct {
	w       io.Writer
	prefix  []byte
	atStart bool
}

// NewPrefixedStreamWriter returns a standardized prefixed writer for
// streams.
func NewPrefixedStreamWriter(w io.Writer, action string, context string) *StreamWriter {
	c_action := color.New(color.BgHiGreen, color.FgBlack).SprintFunc()
	c_context := color.New(color.FgHiWhite).SprintFunc()
	prefix := fmt.Sprintf("%s %s ", c_action(" "+action+" "), c_context(context))
	return NewStreamWriter(w, prefix)
}

// NewStreamWriter returns a wrapper for an io.Writer
func NewStreamWriter(w io.Writer, prefix string) *StreamWriter {
	return &StreamWriter{
		w:       w,
		prefix:  []byte(prefix),
		atStart: true,
	}
}

// Write implements the io.Writer interface for StreamWriter
func (sw *StreamWriter) Write(data []byte) (int, error) {
	var buf bytes.Buffer

	for _, b := range data {
		if sw.atStart {
			buf.Write(sw.prefix)
			sw.atStart = false
		}
		if b == '\n' || b == '\r' {
			sw.atStart = true
		}
		buf.WriteByte(b)
	}

	_, err := sw.w.Write(buf.Bytes())
	return len(data), err
}
