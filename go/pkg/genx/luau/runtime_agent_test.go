package luau

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/genx"
)

func TestNewAgentRuntime(t *testing.T) {
	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	ar := NewAgentRuntime(ctx, mockRT, mockState, &AgentRuntimeConfig{
		AgentName:  "test-agent",
		AgentModel: "qwen-turbo",
	})

	if ar == nil {
		t.Fatal("NewAgentRuntime returned nil")
	}

	// Test inherited ToolRuntime methods
	if ar.AgentName() != "test-agent" {
		t.Errorf("AgentName = %q, want %q", ar.AgentName(), "test-agent")
	}

	if ar.AgentModel() != "qwen-turbo" {
		t.Errorf("AgentModel = %q, want %q", ar.AgentModel(), "qwen-turbo")
	}
}

func TestAgentRuntime_RecvEmit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	ar := NewAgentRuntime(ctx, mockRT, mockState, nil)
	handle := NewAgentHandle(ar)

	// Test in goroutine
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		// Receive input
		input, err := ar.Recv()
		if err != nil {
			t.Errorf("Recv error: %v", err)
			return
		}

		if len(input) != 1 {
			t.Errorf("Input length = %d, want 1", len(input))
			return
		}

		text, ok := input[0].(genx.Text)
		if !ok {
			t.Errorf("Input type = %T, want genx.Text", input[0])
			return
		}

		if string(text) != "Hello" {
			t.Errorf("Input text = %q, want %q", text, "Hello")
			return
		}

		// Emit response
		err = ar.Emit(&genx.MessageChunk{
			Role: genx.RoleModel,
			Part: genx.Text("Hi there!"),
		})
		if err != nil {
			t.Errorf("Emit error: %v", err)
			return
		}

		// Close to signal done
		ar.Close()
	}()

	// Send input
	err := handle.SendText("Hello")
	if err != nil {
		t.Fatalf("SendText error: %v", err)
	}

	// Receive output
	chunk, ok := handle.Next()
	if !ok {
		t.Fatal("Next returned false, expected chunk")
	}

	text, ok := chunk.Part.(genx.Text)
	if !ok {
		t.Fatalf("Chunk Part type = %T, want genx.Text", chunk.Part)
	}

	if string(text) != "Hi there!" {
		t.Errorf("Response text = %q, want %q", text, "Hi there!")
	}

	wg.Wait()
}

func TestAgentRuntime_CollectText(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	ar := NewAgentRuntime(ctx, mockRT, mockState, nil)
	handle := NewAgentHandle(ar)

	go func() {
		// Emit multiple chunks
		ar.Emit(&genx.MessageChunk{Role: genx.RoleModel, Part: genx.Text("Hello")})
		ar.Emit(&genx.MessageChunk{Role: genx.RoleModel, Part: genx.Text(" ")})
		ar.Emit(&genx.MessageChunk{Role: genx.RoleModel, Part: genx.Text("World")})
		ar.Emit(&genx.MessageChunk{Role: genx.RoleModel, Part: genx.Text("!")})
		ar.Close()
	}()

	text, err := handle.CollectText()
	if err != nil {
		t.Fatalf("CollectText error: %v", err)
	}

	if text != "Hello World!" {
		t.Errorf("CollectText = %q, want %q", text, "Hello World!")
	}
}

func TestAgentRuntime_Close(t *testing.T) {
	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	ar := NewAgentRuntime(ctx, mockRT, mockState, nil)

	// Close the runtime
	err := ar.Close()
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}

	// Recv should return ErrAgentClosed
	_, err = ar.Recv()
	if err != ErrAgentClosed {
		t.Errorf("Recv after close error = %v, want ErrAgentClosed", err)
	}

	// Emit should return ErrAgentClosed
	err = ar.Emit(&genx.MessageChunk{})
	if err != ErrAgentClosed {
		t.Errorf("Emit after close error = %v, want ErrAgentClosed", err)
	}

	// Double close should be safe
	err = ar.Close()
	if err != nil {
		t.Errorf("Double close error: %v", err)
	}
}

func TestAgentRuntime_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	ar := NewAgentRuntime(ctx, mockRT, mockState, nil)

	// Cancel the context
	cancel()

	// Give a moment for cancellation to propagate
	time.Sleep(10 * time.Millisecond)

	// Recv should return context error
	_, err := ar.Recv()
	if err != context.Canceled {
		t.Errorf("Recv after cancel error = %v, want context.Canceled", err)
	}
}

func TestAgentHandle_Send(t *testing.T) {
	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	ar := NewAgentRuntime(ctx, mockRT, mockState, nil)
	handle := NewAgentHandle(ar)

	// Start receiver
	received := make(chan genx.Contents, 1)
	go func() {
		input, _ := ar.Recv()
		received <- input
	}()

	// Send contents
	err := handle.Send(genx.Contents{
		genx.Text("Hello"),
		&genx.Blob{MIMEType: "image/png", Data: []byte("fake image")},
	})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}

	// Verify received
	select {
	case input := <-received:
		if len(input) != 2 {
			t.Fatalf("Input length = %d, want 2", len(input))
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for input")
	}
}

func TestAgentHandle_CloseInput(t *testing.T) {
	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	ar := NewAgentRuntime(ctx, mockRT, mockState, nil)
	handle := NewAgentHandle(ar)

	// Close input
	handle.CloseInput()

	// Recv should return ErrAgentClosed (input channel is closed)
	_, err := ar.Recv()
	if err != ErrAgentClosed {
		t.Errorf("Recv after CloseInput error = %v, want ErrAgentClosed", err)
	}
}

func TestAgentRuntime_InheritedMethods(t *testing.T) {
	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	mockRT.GenerateStreamFunc = func(ctx context.Context, model string, mctx genx.ModelContext) (genx.Stream, error) {
		return NewMockTextStream("Hello"), nil
	}

	ar := NewAgentRuntime(ctx, mockRT, mockState, nil)

	// Test Generate (inherited from ToolRuntime)
	result, err := ar.Generate("qwen-turbo", "Say hello")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if result != "Hello" {
		t.Errorf("Generate result = %q, want %q", result, "Hello")
	}

	// Test State operations (inherited)
	ar.StateSet("key", "value")
	v, ok := ar.StateGet("key")
	if !ok || v != "value" {
		t.Errorf("StateGet = %v, %v; want value, true", v, ok)
	}
}

func TestAgentRuntime_ConcurrentEmit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	ar := NewAgentRuntime(ctx, mockRT, mockState, &AgentRuntimeConfig{
		OutputBuffer: 100,
	})
	handle := NewAgentHandle(ar)

	// Emit concurrently
	var wg sync.WaitGroup
	numEmits := 10

	for i := 0; i < numEmits; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ar.Emit(&genx.MessageChunk{
				Role: genx.RoleModel,
				Part: genx.Text("chunk"),
			})
		}(i)
	}

	wg.Wait()
	ar.Close()

	// Collect all chunks
	chunks, err := handle.Collect()
	if err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	if len(chunks) != numEmits {
		t.Errorf("Collected %d chunks, want %d", len(chunks), numEmits)
	}
}
