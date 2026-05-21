package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/user/cc-box/core/webdav"
)

func TestGetDashboardRemoteHeadRetriesTransientFailure(t *testing.T) {
	oldDelay := dashboardRemoteHeadRetryDelay
	dashboardRemoteHeadRetryDelay = 0
	defer func() { dashboardRemoteHeadRetryDelay = oldDelay }()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/HEAD" {
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		if atomic.AddInt32(&attempts, 1) == 1 {
			http.Error(w, "temporary", http.StatusInternalServerError)
			return
		}
		w.Header().Set("ETag", "head-etag")
		_, _ = w.Write([]byte("head-id"))
	}))
	defer server.Close()

	data, etag, err := getDashboardRemoteHead(webdav.NewClient(server.URL, "", ""))
	if err != nil {
		t.Fatalf("getDashboardRemoteHead returned error: %v", err)
	}
	if string(data) != "head-id" || etag != "head-etag" {
		t.Fatalf("data=%q etag=%q, want head-id/head-etag", string(data), etag)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestGetDashboardRemoteHeadDoesNotRetryNotFound(t *testing.T) {
	oldDelay := dashboardRemoteHeadRetryDelay
	dashboardRemoteHeadRetryDelay = time.Hour
	defer func() { dashboardRemoteHeadRetryDelay = oldDelay }()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.NotFound(w, r)
	}))
	defer server.Close()

	_, _, err := getDashboardRemoteHead(webdav.NewClient(server.URL, "", ""))
	if err != webdav.ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
}
