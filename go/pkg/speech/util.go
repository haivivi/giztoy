package speech

import (
	"errors"
	"io"
	"iter"
	"sync"
	"sync/atomic"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/pcm"

	"google.golang.org/api/iterator"
)

// speechConcatenator concatenates multiple speeches from a stream into one.
type speechConcatenator struct {
	mu     sync.Mutex
	speech Speech
	err    error

	speechStream SpeechStream
}

// CollectSpeech collects all speeches from a SpeechStream into a single Speech.
func CollectSpeech(sst SpeechStream) Speech {
	return &speechConcatenator{
		speechStream: sst,
	}
}

func (ch *speechConcatenator) Next() (seg SpeechSegment, err error) {
	if ch.err != nil {
		return nil, ch.err
	}
	defer func() {
		if err != nil {
			ch.err = err
			ch.speechStream.Close()
		}
	}()
	for {
		if ch.speech == nil {
			speech, err := ch.speechStream.Next()
			if err != nil {
				return nil, err
			}
			ch.mu.Lock()
			ch.speech = speech
			ch.mu.Unlock()
		}

		seg, err := ch.speech.Next()
		if err != nil {
			if errors.Is(err, iterator.Done) {
				ch.speech.Close()
				ch.mu.Lock()
				ch.speech = nil
				ch.mu.Unlock()
				continue
			}
			return nil, err
		}
		return seg, nil
	}
}

func (ch *speechConcatenator) Close() error {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if ch.speech != nil {
		ch.speech.Close()
	}
	return ch.speechStream.Close()
}

// CopySpeech copies speech audio to a PCM writer and transcript to a text writer.
// Returns the total duration of the copied audio.
func CopySpeech(pw pcm.Writer, tw io.Writer, speech Speech) (duration time.Duration, err error) {
	first := true
	for seg, err := range Iter(speech) {
		if err != nil {
			return duration, err
		}
		d, err := copySpeechSegment(pw, tw, seg)
		if err != nil {
			return duration, err
		}
		duration += d
		if tw != nil && !first {
			io.WriteString(tw, "\n")
		}
		first = false
	}
	return duration, nil
}

func copySpeechSegment(pw pcm.Writer, tw io.Writer, seg SpeechSegment) (duration time.Duration, err error) {
	defer seg.Close()

	voice := seg.Decode(pcm.L16Mono16K)
	defer voice.Close()

	transcript := seg.Transcribe()
	defer transcript.Close()

	var written atomic.Int64
	defer func() {
		duration = voice.Format().Duration(written.Load())
	}()

	ch := make(chan error, 2)
	go func() {
		if pw == nil {
			voice.Close()
			ch <- nil
			return
		}
		ch <- func() error {
			buf := make([]byte, voice.Format().BytesRate()/10)
			for {
				n, err := voice.Read(buf)
				if err != nil {
					if errors.Is(err, io.EOF) {
						return nil
					}
					return err
				}
				if err := pw.Write(voice.Format().DataChunk(buf[:n])); err != nil {
					return err
				}
				written.Add(int64(n))
			}
		}()
	}()

	go func() {
		if tw == nil {
			transcript.Close()
			ch <- nil
			return
		}
		_, err := io.Copy(tw, transcript)
		ch <- err
	}()

	if err = <-ch; err != nil {
		go func() {
			<-ch
			close(ch)
		}()
		return
	}
	err = <-ch
	close(ch)
	return
}

// NextIter is an interface for types that support Next() iteration.
type NextIter[T any] interface {
	Next() (T, error)
}

// Iter converts a NextIter to an iter.Seq2 for use with range loops.
// The iteration stops when Next() returns iterator.Done.
func Iter[T any](it NextIter[T]) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for {
			el, err := it.Next()
			if err != nil {
				if errors.Is(err, iterator.Done) {
					return
				}
				yield(el, err)
				return
			}
			if !yield(el, nil) {
				return
			}
		}
	}
}
