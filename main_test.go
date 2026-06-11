package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestListAllRepositories_HappyPath(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		page := r.URL.Query().Get("page")

		var repos []*Repository
		var linkHeader string

		switch page {
		case "", "1":
			repos = []*Repository{{ID: 1, Name: "repo1"}}
			linkHeader = fmt.Sprintf("<%s/orgs/test-org/repos?page=2>; rel=\"next\"", "http://"+r.Host)
		case "2":
			repos = []*Repository{{ID: 2, Name: "repo2"}}
			linkHeader = fmt.Sprintf("<%s/orgs/test-org/repos?page=3>; rel=\"next\"", "http://"+r.Host)
		case "3":
			repos = []*Repository{{ID: 3, Name: "repo3"}}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if linkHeader != "" {
			w.Header().Set("Link", linkHeader)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(repos)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	ctx := context.Background()

	repos, err := client.ListAllRepositories(ctx, "test-org", &ListOptions{PerPage: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repos) != 3 {
		t.Errorf("expected 3 repositories, got %d", len(repos))
	}
	if atomic.LoadInt32(&requestCount) != 3 {
		t.Errorf("expected 3 requests, got %d", requestCount)
	}
}

func TestListAllRepositories_ContextCancellation(t *testing.T) {
	var requestCount int32
	ctx, cancel := context.WithCancel(context.Background())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		page := r.URL.Query().Get("page")

		if count == 2 {
			time.Sleep(200 * time.Millisecond)
		}

		var repos []*Repository
		var linkHeader string

		switch page {
		case "", "1":
			repos = []*Repository{{ID: 1, Name: "repo1"}}
			linkHeader = fmt.Sprintf("<%s/orgs/test-org/repos?page=2>; rel=\"next\"", "http://"+r.Host)
		case "2":
			repos = []*Repository{{ID: 2, Name: "repo2"}}
			linkHeader = fmt.Sprintf("<%s/orgs/test-org/repos?page=3>; rel=\"next\"", "http://"+r.Host)
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if linkHeader != "" {
			w.Header().Set("Link", linkHeader)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(repos)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	repos, err := client.ListAllRepositories(ctx, "test-org", &ListOptions{PerPage: 1})
	if err == nil {
		t.Fatal("expected error due to context cancellation, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}

	if count := atomic.LoadInt32(&requestCount); count != 2 {
		t.Errorf("expected 2 requests to be made before cancellation stopped it, got %d", count)
	}

	if len(repos) != 1 {
		t.Errorf("expected 1 repository (accumulated before cancellation), got %d", len(repos))
	}
}

