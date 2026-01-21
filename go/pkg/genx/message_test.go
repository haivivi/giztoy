package genx

import (
	"bytes"
	"testing"
)

func TestRole_String(t *testing.T) {
	tests := []struct {
		role Role
		want string
	}{
		{RoleUser, "user"},
		{RoleModel, "model"},
		{RoleTool, "tool"},
		{Role("custom"), "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.role.String(); got != tt.want {
				t.Errorf("Role.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestText_clone(t *testing.T) {
	original := Text("hello world")
	cloned := original.clone()

	if cloned != original {
		t.Errorf("Text.clone() = %v, want %v", cloned, original)
	}

	// Verify it's the same type
	if _, ok := cloned.(Text); !ok {
		t.Errorf("Text.clone() type = %T, want Text", cloned)
	}
}

func TestBlob_clone(t *testing.T) {
	original := &Blob{
		MIMEType: "image/png",
		Data:     []byte{1, 2, 3, 4, 5},
	}

	cloned := original.clone()
	clonedBlob, ok := cloned.(*Blob)
	if !ok {
		t.Fatalf("Blob.clone() type = %T, want *Blob", cloned)
	}

	if clonedBlob.MIMEType != original.MIMEType {
		t.Errorf("MIMEType = %q, want %q", clonedBlob.MIMEType, original.MIMEType)
	}

	if !bytes.Equal(clonedBlob.Data, original.Data) {
		t.Errorf("Data = %v, want %v", clonedBlob.Data, original.Data)
	}

	// Verify it's a deep copy (modifying original doesn't affect clone)
	original.Data[0] = 99
	if clonedBlob.Data[0] == 99 {
		t.Error("clone should be independent of original")
	}
}

func TestMessageChunk_Clone(t *testing.T) {
	original := &MessageChunk{
		Role: RoleUser,
		Name: "test",
		Part: Text("hello"),
	}

	cloned := original.Clone()

	if cloned.Role != original.Role {
		t.Errorf("Role = %v, want %v", cloned.Role, original.Role)
	}

	if cloned.Name != original.Name {
		t.Errorf("Name = %v, want %v", cloned.Name, original.Name)
	}

	if cloned.Part != original.Part {
		t.Errorf("Part = %v, want %v", cloned.Part, original.Part)
	}
}

func TestMessageChunk_Clone_WithBlob(t *testing.T) {
	original := &MessageChunk{
		Role: RoleModel,
		Name: "assistant",
		Part: &Blob{
			MIMEType: "audio/mp3",
			Data:     []byte{10, 20, 30},
		},
	}

	cloned := original.Clone()

	// Verify blob is deep copied
	originalBlob := original.Part.(*Blob)
	clonedBlob := cloned.Part.(*Blob)

	if clonedBlob.MIMEType != originalBlob.MIMEType {
		t.Errorf("Blob.MIMEType = %q, want %q", clonedBlob.MIMEType, originalBlob.MIMEType)
	}

	// Modify original, verify clone is unaffected
	originalBlob.Data[0] = 99
	if clonedBlob.Data[0] == 99 {
		t.Error("cloned blob should be independent of original")
	}
}

func TestContents_isPayload(t *testing.T) {
	var c Contents = []Part{Text("test")}
	// This should compile - Contents implements Payload
	var _ Payload = c
	c.isPayload() // Just verify it doesn't panic
}

func TestToolCall_isPayload(t *testing.T) {
	tc := &ToolCall{ID: "test-id"}
	// This should compile - ToolCall implements Payload
	var _ Payload = tc
	tc.isPayload() // Just verify it doesn't panic
}

func TestToolResult_isPayload(t *testing.T) {
	tr := &ToolResult{ID: "test-id", Result: "result"}
	// This should compile - ToolResult implements Payload
	var _ Payload = tr
	tr.isPayload() // Just verify it doesn't panic
}

func TestBlob_isPart(t *testing.T) {
	b := &Blob{MIMEType: "text/plain", Data: []byte("test")}
	// This should compile - Blob implements Part
	var _ Part = b
	b.isPart() // Just verify it doesn't panic
}

func TestText_isPart(t *testing.T) {
	text := Text("hello")
	// This should compile - Text implements Part
	var _ Part = text
	text.isPart() // Just verify it doesn't panic
}
