package main

import (
	"net/http"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

type testHandler struct{}

func (h *testHandler) ServeHTTP(_ http.ResponseWriter, _ *http.Request) {}
