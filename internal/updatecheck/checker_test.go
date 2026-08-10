package updatecheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCheckerResolvesRepositoryByIDBeforeLatestRelease(t *testing.T) {
	var requestedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/repositories/1326685411":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"full_name": "renamed-owner/go-script-sql-runner",
			})
		case "/repos/renamed-owner/go-script-sql-runner/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "build-42",
				"html_url": "https://github.com/renamed-owner/go-script-sql-runner/releases/tag/build-42",
				"assets": []map[string]string{
					{
						"name":                 "go-script-sql-runner.exe",
						"browser_download_url": "https://github.com/renamed-owner/go-script-sql-runner/releases/download/build-42/go-script-sql-runner.exe",
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	checker := &Checker{
		client:       server.Client(),
		apiBaseURL:   server.URL,
		repositoryID: 1326685411,
		currentTag:   "build-41",
	}

	result, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	wantPaths := []string{
		"/repositories/1326685411",
		"/repos/renamed-owner/go-script-sql-runner/releases/latest",
	}
	if !reflect.DeepEqual(requestedPaths, wantPaths) {
		t.Fatalf("requested paths = %v, want %v", requestedPaths, wantPaths)
	}
	if !result.Available {
		t.Fatal("expected update to be available")
	}
	if result.LatestTag != "build-42" {
		t.Fatalf("LatestTag = %q, want build-42", result.LatestTag)
	}
	if result.DownloadURL == "" {
		t.Fatal("expected executable download URL")
	}
}

func TestCheckerDoesNotMarkCurrentBuildAsUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repositories/1326685411":
			_, _ = w.Write([]byte(`{"full_name":"owner/go-script-sql-runner"}`))
		case "/repos/owner/go-script-sql-runner/releases/latest":
			_, _ = w.Write([]byte(`{"tag_name":"build-42","html_url":"https://example.invalid/build-42","assets":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	checker := &Checker{
		client:       server.Client(),
		apiBaseURL:   server.URL,
		repositoryID: 1326685411,
		currentTag:   "build-42",
	}

	result, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Available {
		t.Fatal("did not expect current build to be marked as an update")
	}
}
