package proxy

import (
	"encoding/json"
	"net/http/httptest"
)

// streamingRecorder is a custom ResponseRecorder that implements http.Flusher
// for testing streaming responses
type streamingRecorder struct {
	*httptest.ResponseRecorder
}

func (sr *streamingRecorder) Flush() {
	// 实现Flusher接口
}

// badReader is a custom reader that always returns an error
// for testing error cases
type badReader struct{}

func (br *badReader) Read([]byte) (int, error) {
	return 0, &json.SyntaxError{Offset: 123}
}
