package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// helper to create test server
func setupServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/compute", computeHandler)
	return httptest.NewServer(mux)
}


// ---------- HEALTH TEST ----------

func TestHealth_OK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	if w.Body.String() != "OK" {
		t.Errorf("expected OK, got %s", w.Body.String())
	}
}

func TestHealth_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// ---------- COMPUTE TEST ----------

func TestCompute_Add(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/compute?op=add&a=5&b=7", nil)
	w := httptest.NewRecorder()

	computeHandler(w, req)

	var res Response
	json.NewDecoder(w.Body).Decode(&res)

	if res.Result != 12 {
		t.Errorf("expected 12, got %f", res.Result)
	}
}

func TestCompute_DivisionByZero(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/compute?op=div&a=5&b=0", nil)
	w := httptest.NewRecorder()

	computeHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCompute_InvalidOp(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/compute?op=xxx&a=5&b=2", nil)
	w := httptest.NewRecorder()

	computeHandler(w, req)

	var res Response
	json.NewDecoder(w.Body).Decode(&res)

	if res.Error == "" {
		t.Errorf("expected error for invalid op")
	}
}

func TestCompute_MissingParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/compute?op=add&a=5", nil)
	w := httptest.NewRecorder()

	computeHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCompute_InvalidNumber(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/compute?op=add&a=abc&b=2", nil)
	w := httptest.NewRecorder()

	computeHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCompute_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/compute", nil)
	w := httptest.NewRecorder()

	computeHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

//
// ✅ SCENARIO 1: health + successful compute
//
func TestScenario_HealthyAndCompute(t *testing.T) {
	server := setupServer()
	defer server.Close()

	// health check
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("health expected 200 got %d", resp.StatusCode)
	}

	// compute
	resp, err = http.Get(server.URL + "/compute?op=add&a=5&b=7")
	if err != nil {
		t.Fatal(err)
	}

	var res Response
	json.NewDecoder(resp.Body).Decode(&res)

	if res.Result != 12 {
		t.Fatalf("expected 12 got %f", res.Result)
	}
}

//
// ✅ SCENARIO 2: multiple sequential requests (stateless)
//
func TestScenario_MultipleRequests(t *testing.T) {
	server := setupServer()
	defer server.Close()

	tests := []struct {
		url    string
		expect float64
	}{
		{"/compute?op=add&a=1&b=2", 3},
		{"/compute?op=sub&a=5&b=3", 2},
		{"/compute?op=mul&a=2&b=4", 8},
	}

	for _, tc := range tests {
		resp, err := http.Get(server.URL + tc.url)
		if err != nil {
			t.Fatal(err)
		}

		var res Response
		json.NewDecoder(resp.Body).Decode(&res)

		if res.Result != tc.expect {
			t.Fatalf("expected %f got %f", tc.expect, res.Result)
		}
	}
}

//
// ✅ SCENARIO 3: user errors
//
func TestScenario_UserErrors(t *testing.T) {
	server := setupServer()
	defer server.Close()

	cases := []string{
		"/compute?op=add&a=5",              // missing param
		"/compute?op=xxx&a=5&b=2",          // invalid op
		"/compute?op=add&a=abc&b=2",        // invalid number
	}

	for _, url := range cases {
		resp, err := http.Get(server.URL + url)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s, got %d", url, resp.StatusCode)
		}
	}
}

//
// ✅ SCENARIO 4: division by zero
//
func TestScenario_DivisionByZero(t *testing.T) {
	server := setupServer()
	defer server.Close()

	resp, err := http.Get(server.URL + "/compute?op=div&a=5&b=0")
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", resp.StatusCode)
	}
}

//
// ✅ SCENARIO 5: method validation
//
func TestScenario_MethodNotAllowed(t *testing.T) {
	server := setupServer()
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/compute", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 got %d", resp.StatusCode)
	}
}