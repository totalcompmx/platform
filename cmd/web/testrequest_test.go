package main

import (
	"net/http"
	"net/url"
	"testing"
)

func newTestRequest(t *testing.T, method, path string) *http.Request {
	req, err := http.NewRequest(method, path, nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Form = url.Values{}
	req.PostForm = url.Values{}

	req.Header.Set("Sec-Fetch-Site", "same-origin")
	return req
}
