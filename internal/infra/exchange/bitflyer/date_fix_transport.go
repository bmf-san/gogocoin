package bitflyer

import (
	"bytes"
	"io"
	"net/http"
	"regexp"
)

// dateWithoutTZ matches ISO 8601 datetime strings that lack a timezone suffix.
// bitFlyer's private API returns timestamps like "2026-03-31T13:08:33" (no 'Z'
// or offset), which encoding/json cannot unmarshal into time.Time. We fix them
// by appending 'Z' (UTC) before the JSON decoder runs.
var dateWithoutTZ = regexp.MustCompile(`"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})"`)

// emptyDateField matches JSON date/timestamp fields whose value is an empty
// string (e.g. "expire_date":""). bitFlyer returns "" for optional datetime
// fields that have no value, but encoding/json cannot unmarshal "" into
// time.Time. Replace them with JSON null so the *time.Time pointer stays nil.
var emptyDateField = regexp.MustCompile(`"([^"]*(?:date|timestamp)[^"]*)"(\s*:\s*)""`)

// dateFixingTransport is an http.RoundTripper that appends 'Z' to timezone-less
// datetime strings in JSON response bodies from the bitFlyer API.
type dateFixingTransport struct {
	base http.RoundTripper
}

// newDateFixingTransport returns a new dateFixingTransport wrapping base.
// If base is nil, http.DefaultTransport is used.
func newDateFixingTransport(base http.RoundTripper) *dateFixingTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &dateFixingTransport{base: base}
}

// RoundTrip executes the request and rewrites any timezone-naïve datetime
// values in the JSON response body to RFC3339 UTC format.
func (t *dateFixingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Only process JSON responses.
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		if !containsJSON(ct) {
			return resp, nil
		}
	}

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}

	// Replace "2006-01-02T15:04:05" → "2006-01-02T15:04:05Z"
	fixed := dateWithoutTZ.ReplaceAll(body, []byte(`"$1Z"`))
	// Replace "expire_date":"" → "expire_date":null  (and similar date fields)
	fixed = emptyDateField.ReplaceAll(fixed, []byte(`"$1"${2}null`))

	resp.Body = io.NopCloser(bytes.NewReader(fixed))
	resp.ContentLength = int64(len(fixed))
	return resp, nil
}

func containsJSON(ct string) bool {
	return bytes.Contains([]byte(ct), []byte("json"))
}
