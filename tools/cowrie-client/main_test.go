package main

import (
	"bytes"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestConfigureInteractiveOutputSeparatesConcurrentStreams(t *testing.T) {
	t.Parallel()

	session := new(ssh.Session)
	stdout, stderr := configureInteractiveOutput(session)

	if session.Stdout == session.Stderr {
		t.Fatal("stdout and stderr must not share a mutable buffer")
	}
	if session.Stdout != stdout || session.Stderr != stderr {
		t.Fatal("session streams do not use the returned buffers")
	}

	const writes = 256
	stdoutChunk := bytes.Repeat([]byte("stdout\n"), 64)
	stderrChunk := bytes.Repeat([]byte("stderr\n"), 64)

	var writers sync.WaitGroup
	writers.Add(2)
	go writeChunks(t, &writers, session.Stdout, stdoutChunk, writes)
	go writeChunks(t, &writers, session.Stderr, stderrChunk, writes)
	writers.Wait()

	if want := len(stdoutChunk) * writes; stdout.Len() != want {
		t.Fatalf("stdout length = %d, want %d", stdout.Len(), want)
	}
	if want := len(stderrChunk) * writes; stderr.Len() != want {
		t.Fatalf("stderr length = %d, want %d", stderr.Len(), want)
	}
	if want := bytes.Repeat(stdoutChunk, writes); !bytes.Equal(stdout.Bytes(), want) {
		t.Fatal("stdout content was corrupted")
	}
	if want := bytes.Repeat(stderrChunk, writes); !bytes.Equal(stderr.Bytes(), want) {
		t.Fatal("stderr content was corrupted")
	}
}

func writeChunks(t *testing.T, writers *sync.WaitGroup, output interface{ Write([]byte) (int, error) }, chunk []byte, count int) {
	t.Helper()
	defer writers.Done()

	for range count {
		if _, err := output.Write(chunk); err != nil {
			t.Errorf("write output: %v", err)
			return
		}
	}
}
