// Package ogg provides Go bindings for libogg.
//
// libogg is the reference implementation of the Ogg container format.
// This package provides low-level access to Ogg sync, stream, page and packet operations.
package ogg

/*
#cgo pkg-config: ogg
#include <ogg/ogg.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"unsafe"
)

// Page header type flags
const (
	// Continued indicates this page contains data from a packet continued from the previous page
	Continued = 0x01
	// BOS indicates beginning of stream
	BOS = 0x02
	// EOS indicates end of stream
	EOS = 0x04
)

// Page represents an Ogg page.
type Page struct {
	page C.ogg_page
}

// Header returns the page header data.
func (p *Page) Header() []byte {
	return C.GoBytes(unsafe.Pointer(p.page.header), C.int(p.page.header_len))
}

// Body returns the page body data.
func (p *Page) Body() []byte {
	return C.GoBytes(unsafe.Pointer(p.page.body), C.int(p.page.body_len))
}

// SerialNo returns the stream serial number.
func (p *Page) SerialNo() int32 {
	return int32(C.ogg_page_serialno(&p.page))
}

// PageNo returns the page sequence number.
func (p *Page) PageNo() int64 {
	return int64(C.ogg_page_pageno(&p.page))
}

// IsBOS returns true if this is a beginning of stream page.
func (p *Page) IsBOS() bool {
	return C.ogg_page_bos(&p.page) != 0
}

// IsEOS returns true if this is an end of stream page.
func (p *Page) IsEOS() bool {
	return C.ogg_page_eos(&p.page) != 0
}

// GranulePos returns the granule position.
func (p *Page) GranulePos() int64 {
	return int64(C.ogg_page_granulepos(&p.page))
}

// Packets returns the number of complete packets in this page.
func (p *Page) Packets() int {
	return int(C.ogg_page_packets(&p.page))
}

// Packet represents an Ogg packet.
// Data is stored as a Go slice to avoid CGO pointer issues.
type Packet struct {
	data       []byte
	granulePos int64
	packetNo   int64
	bos        bool
	eos        bool
}

// Data returns the packet data.
func (p *Packet) Data() []byte {
	return p.data
}

// Bytes returns the packet length.
func (p *Packet) Bytes() int64 {
	return int64(len(p.data))
}

// BOS returns true if this is a beginning of stream packet.
func (p *Packet) BOS() bool {
	return p.bos
}

// EOS returns true if this is an end of stream packet.
func (p *Packet) EOS() bool {
	return p.eos
}

// GranulePos returns the granule position.
func (p *Packet) GranulePos() int64 {
	return p.granulePos
}

// PacketNo returns the packet sequence number.
func (p *Packet) PacketNo() int64 {
	return p.packetNo
}
