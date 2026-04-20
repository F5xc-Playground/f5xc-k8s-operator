// Package testutil provides a reusable httptest-based fake that mimics the
// F5 XC REST API for integration testing. It supports CRUD operations,
// request recording, and injectable errors.
package testutil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// StoredObject represents a single F5 XC object held in the fake server's
// in-memory store.
type StoredObject struct {
	Metadata       map[string]interface{} `json:"metadata"`
	SystemMetadata map[string]interface{} `json:"system_metadata,omitempty"`
	Spec           map[string]interface{} `json:"spec"`
}

// RecordedRequest captures the method, path, and body of every request the
// fake server receives. Callers use this to assert on request behaviour.
type RecordedRequest struct {
	Method string
	Path   string
	Body   json.RawMessage
}

// ErrorSpec describes a synthetic error that the fake server should return for
// a particular operation. Times controls how many times the error fires:
// 0 means forever, >0 counts down and the entry is removed when exhausted.
type ErrorSpec struct {
	StatusCode int
	Body       string
	Times      int // 0 = forever, >0 = count down
}

// FakeXCServer is an httptest-based server that mimics the F5 XC REST API.
// The zero value is not usable — use NewFakeXCServer() to create one.
type FakeXCServer struct {
	// Server is the underlying httptest.Server. Callers may use Server.URL
	// but should prefer calling URL() instead.
	Server *httptest.Server

	mu       sync.Mutex
	objects  map[string]StoredObject // keyed by "resource/namespace/name"
	requests []RecordedRequest
	errors   map[string]*errorEntry // keyed by "METHOD resource/namespace/name"
}

// errorEntry wraps an ErrorSpec so the fake server can decrement the remaining
// count in a thread-safe way.
type errorEntry struct {
	spec      ErrorSpec
	remaining int // -1 means infinite (spec.Times == 0)
}

// NewFakeXCServer creates and starts a new FakeXCServer. Remember to call
// Close() when the test is done.
func NewFakeXCServer() *FakeXCServer {
	f := &FakeXCServer{
		objects: make(map[string]StoredObject),
		errors:  make(map[string]*errorEntry),
	}
	f.Server = httptest.NewServer(http.HandlerFunc(f.handle))
	return f
}

// Close shuts down the underlying httptest.Server.
func (f *FakeXCServer) Close() {
	f.Server.Close()
}

// URL returns the base URL of the fake server.
func (f *FakeXCServer) URL() string {
	return f.Server.URL
}

// InjectError registers a synthetic error for the given operation. The error
// key is built from "METHOD resource/namespace/name". If spec.Times > 0 the
// error fires that many times then is removed automatically; if spec.Times == 0
// the error fires on every matching request until ClearErrors is called.
func (f *FakeXCServer) InjectError(method, resource, namespace, name string, spec ErrorSpec) {
	key := errorKey(method, resource, namespace, name)
	entry := &errorEntry{spec: spec}
	if spec.Times == 0 {
		entry.remaining = -1 // infinite
	} else {
		entry.remaining = spec.Times
	}
	f.mu.Lock()
	f.errors[key] = entry
	f.mu.Unlock()
}

// ClearErrors removes all injected errors.
func (f *FakeXCServer) ClearErrors() {
	f.mu.Lock()
	f.errors = make(map[string]*errorEntry)
	f.mu.Unlock()
}

// Requests returns a copy of every request recorded since the server was
// started (or since the last ClearRequests call).
func (f *FakeXCServer) Requests() []RecordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]RecordedRequest, len(f.requests))
	copy(cp, f.requests)
	return cp
}

// ClearRequests discards all recorded requests.
func (f *FakeXCServer) ClearRequests() {
	f.mu.Lock()
	f.requests = nil
	f.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func errorKey(method, resource, namespace, name string) string {
	return fmt.Sprintf("%s %s/%s/%s", strings.ToUpper(method), resource, namespace, name)
}

func objectKey(resource, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s", resource, namespace, name)
}

// handle is the single HTTP handler wired to the httptest.Server. All routing,
// recording, error injection, and CRUD logic lives here.
func (f *FakeXCServer) handle(w http.ResponseWriter, r *http.Request) {
	// Parse the URL: /api/config/namespaces/{ns}/{resource}[/{name}]
	// Split gives: ["", "api", "config", "namespaces", ns, resource, name?]
	parts := strings.Split(r.URL.Path, "/")
	// Minimum valid path has 6 parts (no name): ["","api","config","namespaces",ns,resource]
	if len(parts) < 6 || parts[1] != "api" || parts[2] != "config" || parts[3] != "namespaces" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	ns := parts[4]
	resource := parts[5]
	name := ""
	if len(parts) >= 7 {
		name = parts[6]
	}

	// Read the request body (may be empty for GET/DELETE).
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "reading body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Record the request.
	rec := RecordedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
	}
	if len(bodyBytes) > 0 {
		rec.Body = json.RawMessage(bodyBytes)
	}

	f.mu.Lock()
	f.requests = append(f.requests, rec)

	// Check for injected errors before doing anything else.
	ekey := errorKey(r.Method, resource, ns, name)
	if entry, ok := f.errors[ekey]; ok {
		if entry.remaining > 0 {
			entry.remaining--
			if entry.remaining == 0 {
				delete(f.errors, ekey)
			}
		}
		statusCode := entry.spec.StatusCode
		body := entry.spec.Body
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
		return
	}
	f.mu.Unlock()

	// Dispatch to the correct CRUD handler.
	switch r.Method {
	case http.MethodPost:
		f.handleCreate(w, resource, ns, name, bodyBytes)
	case http.MethodGet:
		if name != "" {
			f.handleGet(w, resource, ns, name)
		} else {
			f.handleList(w, resource, ns)
		}
	case http.MethodPut:
		f.handleReplace(w, resource, ns, name, bodyBytes)
	case http.MethodDelete:
		f.handleDelete(w, resource, ns, name)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCreate stores a new object and returns it with server-assigned
// metadata. Returns 409 if the object already exists.
func (f *FakeXCServer) handleCreate(w http.ResponseWriter, resource, ns, name string, body []byte) {
	// Unmarshal the incoming payload.
	var obj StoredObject
	if len(body) > 0 {
		if err := json.Unmarshal(body, &obj); err != nil {
			http.Error(w, "bad JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Derive the name from the body metadata if not in the URL.
	if name == "" {
		if obj.Metadata != nil {
			if n, ok := obj.Metadata["name"].(string); ok {
				name = n
			}
		}
	}

	key := objectKey(resource, ns, name)

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.objects[key]; exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"object already exists"}`))
		return
	}

	// Ensure metadata map exists and populate server-assigned fields.
	if obj.Metadata == nil {
		obj.Metadata = make(map[string]interface{})
	}
	obj.Metadata["name"] = name
	obj.Metadata["namespace"] = ns
	obj.Metadata["resource_version"] = "1"

	// Populate fake system_metadata.
	now := time.Now().UTC().Format(time.RFC3339)
	uid := fmt.Sprintf("fake-uid-%s-%s-%s", ns, resource, name)
	obj.SystemMetadata = map[string]interface{}{
		"uid":                    uid,
		"creation_timestamp":     now,
		"modification_timestamp": now,
		"tenant":                 "fake-tenant",
	}

	f.objects[key] = obj
	writeJSON(w, http.StatusOK, obj)
}

// handleGet returns the object identified by (resource, ns, name) or 404.
func (f *FakeXCServer) handleGet(w http.ResponseWriter, resource, ns, name string) {
	key := objectKey(resource, ns, name)

	f.mu.Lock()
	obj, exists := f.objects[key]
	f.mu.Unlock()

	if !exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
		return
	}

	writeJSON(w, http.StatusOK, obj)
}

// handleList returns all objects whose key starts with "resource/namespace/".
func (f *FakeXCServer) handleList(w http.ResponseWriter, resource, ns string) {
	prefix := fmt.Sprintf("%s/%s/", resource, ns)

	f.mu.Lock()
	items := make([]StoredObject, 0)
	for k, v := range f.objects {
		if strings.HasPrefix(k, prefix) {
			items = append(items, v)
		}
	}
	f.mu.Unlock()

	type listResponse struct {
		Items []StoredObject `json:"items"`
	}
	writeJSON(w, http.StatusOK, listResponse{Items: items})
}

// handleReplace updates an existing object or returns 404.
func (f *FakeXCServer) handleReplace(w http.ResponseWriter, resource, ns, name string, body []byte) {
	key := objectKey(resource, ns, name)

	var obj StoredObject
	if len(body) > 0 {
		if err := json.Unmarshal(body, &obj); err != nil {
			http.Error(w, "bad JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.objects[key]; !exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
		return
	}

	// Ensure metadata map exists and update server-assigned fields.
	if obj.Metadata == nil {
		obj.Metadata = make(map[string]interface{})
	}
	obj.Metadata["name"] = name
	obj.Metadata["namespace"] = ns

	// Update system_metadata timestamps.
	now := time.Now().UTC().Format(time.RFC3339)
	uid := fmt.Sprintf("fake-uid-%s-%s-%s", ns, resource, name)
	obj.SystemMetadata = map[string]interface{}{
		"uid":                    uid,
		"modification_timestamp": now,
		"tenant":                 "fake-tenant",
	}

	f.objects[key] = obj
	writeJSON(w, http.StatusOK, obj)
}

// handleDelete removes the object identified by (resource, ns, name) or 404.
func (f *FakeXCServer) handleDelete(w http.ResponseWriter, resource, ns, name string) {
	key := objectKey(resource, ns, name)

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.objects[key]; !exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
		return
	}

	delete(f.objects, key)
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// writeJSON marshals v to JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "marshalling response: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
