package llm

import (
	"strings"
	"testing"
)

func TestDecodeOpenAIStream(t *testing.T) {
	stream := strings.NewReader(strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"hel"}}]}`,
		`data: {"choices":[{"delta":{"content":"lo"}}]}`,
		`data: [DONE]`,
		``,
	}, "\n"))

	var got strings.Builder
	if err := decodeOpenAIStream(stream, func(chunk string) error {
		got.WriteString(chunk)
		return nil
	}); err != nil {
		t.Fatalf("decode stream failed: %v", err)
	}

	if got.String() != "hello" {
		t.Fatalf("expected concatenated chunks, got %q", got.String())
	}
}

func TestDecodeOpenAIStreamErrorPayload(t *testing.T) {
	stream := strings.NewReader(`data: {"error":{"message":"bad request","type":"invalid_request_error"}}`)

	err := decodeOpenAIStream(stream, func(chunk string) error {
		t.Fatalf("handler should not be called for error payload")
		return nil
	})
	if err == nil {
		t.Fatal("expected error payload to return an error")
	}
}
