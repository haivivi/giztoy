package runtime

import (
	"sync"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

func TestToolContext(t *testing.T) {
	// Create Luau state
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	// Create runtime with tool context
	rt := New(state, nil)
	tc := rt.CreateToolContext()

	// Set input
	tc.SetInput(map[string]any{
		"name":  "world",
		"count": 42,
	})

	// Register functions
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	// Run tool script
	err = state.DoString(`
local input = rt:input()
local result = "Hello, " .. input.name .. "! Count: " .. input.count
rt:output({ message = result }, nil)
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	// Check output
	output, err := tc.GetOutput()
	if err != nil {
		t.Fatalf("GetOutput failed: %v", err)
	}

	outputMap, ok := output.(map[string]any)
	if !ok {
		t.Fatalf("output is not a map: %T", output)
	}

	msg, ok := outputMap["message"].(string)
	if !ok {
		t.Fatalf("message is not a string: %T", outputMap["message"])
	}

	expected := "Hello, world! Count: 42"
	if msg != expected {
		t.Errorf("message = %q, want %q", msg, expected)
	}
}

func TestToolContextError(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	tc := rt.CreateToolContext()
	tc.SetInput("test")

	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	// Run script that outputs an error
	err = state.DoString(`
rt:output(nil, "something went wrong")
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	_, outputErr := tc.GetOutput()
	if outputErr == nil {
		t.Error("expected error, got nil")
	}
	if outputErr.Error() != "something went wrong" {
		t.Errorf("error = %q, want %q", outputErr.Error(), "something went wrong")
	}
}

func TestToolContextNoOutput(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	tc := rt.CreateToolContext()

	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	// Run script that doesn't call output
	err = state.DoString(`
local x = 1 + 1
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	_, err = tc.GetOutput()
	if err != ErrNoOutput {
		t.Errorf("GetOutput error = %v, want ErrNoOutput", err)
	}
}

func TestAgentContext(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	ac := rt.CreateAgentContext(nil)

	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	// Run agent script in goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer ac.Close()

		err := state.DoString(`
-- Simple echo agent: recv one message and emit response
local chunk, err = rt:recv()
if err then return end
rt:emit({ part = { type = "text", value = "Echo received" } })
`)
		if err != nil {
			t.Errorf("DoString failed: %v", err)
		}
	}()

	// Send input
	time.Sleep(10 * time.Millisecond) // Give goroutine time to start
	if err := ac.Send(&MessageChunk{Part: "Hello"}); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Read output
	select {
	case chunk := <-ac.Output():
		if chunk == nil {
			t.Error("received nil chunk")
		}
		// Just verify we got a chunk - the structure is validated by type system
	case <-time.After(time.Second):
		t.Error("timeout waiting for output")
	}

	wg.Wait()
}

func TestAgentContextMultipleMessages(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	ac := rt.CreateAgentContext(&AgentContextConfig{
		InputBufferSize:  5,
		OutputBufferSize: 10,
	})

	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	// Run agent that processes multiple messages
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer ac.Close()

		err := state.DoString(`
local count = 0
while count < 3 do
    local chunk, err = rt:recv()
    if err then break end
    count = count + 1
    rt:emit({ part = { type = "text", value = "Got message " .. count } })
end
`)
		if err != nil {
			t.Errorf("DoString failed: %v", err)
		}
	}()

	// Send 3 messages
	time.Sleep(10 * time.Millisecond)
	for i := 0; i < 3; i++ {
		if err := ac.SendText("message"); err != nil {
			t.Fatalf("SendText %d failed: %v", i, err)
		}
	}

	// Read 3 outputs
	for i := 0; i < 3; i++ {
		select {
		case chunk := <-ac.Output():
			if chunk == nil {
				t.Errorf("received nil chunk %d", i)
			}
		case <-time.After(time.Second):
			t.Errorf("timeout waiting for output %d", i)
		}
	}

	wg.Wait()
}

func TestAgentContextClose(t *testing.T) {
	ac := NewAgentContext(nil)

	// Close should be idempotent
	if err := ac.Close(); err != nil {
		t.Errorf("first Close failed: %v", err)
	}
	if err := ac.Close(); err != nil {
		t.Errorf("second Close failed: %v", err)
	}

	// Send should fail after close
	if err := ac.Send(&MessageChunk{}); err != ErrAgentClosed {
		t.Errorf("Send after close error = %v, want ErrAgentClosed", err)
	}

	if !ac.IsClosed() {
		t.Error("IsClosed should return true")
	}
}

func TestToolContextReset(t *testing.T) {
	tc := NewToolContext()

	tc.SetInput("test")
	tc.output = "result"
	tc.outputSet = true

	tc.Reset()

	if tc.input != nil {
		t.Error("input should be nil after reset")
	}
	if tc.output != nil {
		t.Error("output should be nil after reset")
	}
	if tc.outputSet {
		t.Error("outputSet should be false after reset")
	}
}
