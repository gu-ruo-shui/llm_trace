package proxy

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

// responseContentEncodings returns the Content-Encoding values in the order
// listed by the server. HTTP applies encodings in this order, so decoding a
// full body must happen in reverse order.
func responseContentEncodings(resp *http.Response) []string {
	if resp == nil {
		return nil
	}

	var encodings []string
	for _, value := range resp.Header.Values("Content-Encoding") {
		for _, part := range strings.Split(value, ",") {
			encoding := strings.ToLower(strings.TrimSpace(part))
			if encoding == "" || encoding == "identity" {
				continue
			}
			encodings = append(encodings, encoding)
		}
	}
	return encodings
}

func decodeResponseBody(resp *http.Response, body []byte) ([]byte, bool, error) {
	encodings := responseContentEncodings(resp)
	if len(encodings) == 0 {
		return body, false, nil
	}

	decoded := body
	for i := len(encodings) - 1; i >= 0; i-- {
		var err error
		decoded, err = decodeBodyByEncoding(decoded, encodings[i])
		if err != nil {
			return body, false, err
		}
	}

	return decoded, true, nil
}

func decodeBodyByEncoding(body []byte, encoding string) ([]byte, error) {
	switch encoding {
	case "gzip", "x-gzip":
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	case "deflate":
		// Most HTTP deflate responses use a zlib wrapper. Some servers send raw
		// deflate, so retry with compress/flate when zlib decoding fails.
		reader, err := zlib.NewReader(bytes.NewReader(body))
		if err == nil {
			defer reader.Close()
			return io.ReadAll(reader)
		}

		rawReader := flate.NewReader(bytes.NewReader(body))
		defer rawReader.Close()
		return io.ReadAll(rawReader)
	default:
		return nil, fmt.Errorf("unsupported content-encoding %q", encoding)
	}
}

// newDecodedResponseReadCloser wraps a streaming response body in a decoder
// when the Content-Encoding is supported. Unsupported encodings are left raw so
// the proxy can still pass bytes through to the client unchanged.
func newDecodedResponseReadCloser(resp *http.Response) (io.ReadCloser, bool, error) {
	encodings := responseContentEncodings(resp)
	if len(encodings) == 0 {
		return resp.Body, false, nil
	}

	// Streaming wrapper support is intentionally conservative. The real issue in
	// this project is gzip-compressed SSE; stacked encodings are rare and are kept
	// as raw pass-through bytes rather than risking a broken proxy response.
	if len(encodings) != 1 {
		return resp.Body, false, nil
	}

	switch encodings[0] {
	case "gzip", "x-gzip":
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, false, err
		}
		return reader, true, nil
	case "deflate":
		reader, err := zlib.NewReader(resp.Body)
		if err != nil {
			return nil, false, err
		}
		return reader, true, nil
	default:
		return resp.Body, false, nil
	}
}

func copyResponseHeaders(dst, src http.Header, decoded bool) {
	for key, values := range src {
		if decoded && shouldDropDecodedHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func shouldDropDecodedHeader(key string) bool {
	switch http.CanonicalHeaderKey(key) {
	case "Content-Encoding", "Content-Length", "Content-Md5":
		return true
	default:
		return false
	}
}

func responseForBodyEncoding(resp *http.Response, decoded bool) *http.Response {
	if resp == nil || !decoded {
		return resp
	}

	clone := new(http.Response)
	*clone = *resp
	clone.Header = resp.Header.Clone()
	for key := range clone.Header {
		if shouldDropDecodedHeader(key) {
			clone.Header.Del(key)
		}
	}
	return clone
}

func formatResponseBodyForLog(resp *http.Response, body []byte) string {
	decodedBody, decoded, err := decodeResponseBody(resp, body)
	if err == nil && decoded {
		body = decodedBody
	} else if err != nil {
		return base64ResponseBodyForLog(resp, body, err.Error())
	}

	if utf8.Valid(body) {
		return string(body)
	}

	return base64ResponseBodyForLog(resp, body, "response body is not valid UTF-8")
}

func base64ResponseBodyForLog(resp *http.Response, body []byte, reason string) string {
	contentType := ""
	contentEncoding := ""
	if resp != nil {
		contentType = resp.Header.Get("Content-Type")
		contentEncoding = resp.Header.Get("Content-Encoding")
	}

	return fmt.Sprintf(
		"[base64 response body; reason=%s; content_type=%q; content_encoding=%q; length=%d; data=%s]",
		reason,
		contentType,
		contentEncoding,
		len(body),
		base64.StdEncoding.EncodeToString(body),
	)
}
