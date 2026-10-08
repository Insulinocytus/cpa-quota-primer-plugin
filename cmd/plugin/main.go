package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct { void* ptr; size_t len; } cliproxy_buffer;
typedef int (*cliproxy_host_call_fn)(void*, const char*, const uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_host_free_fn)(void*, size_t);
typedef struct {
    uint32_t abi_version;
    void* host_ctx;
    cliproxy_host_call_fn call;
    cliproxy_host_free_fn free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);
typedef struct {
    uint32_t abi_version;
    cliproxy_plugin_call_fn call;
    cliproxy_plugin_free_fn free_buffer;
    cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);

// Copied by value: the host frees its table once shutdown returns, while a
// detached host.model.execute (see hostExecute) may still be returning.
static cliproxy_host_api stored_host;

static void store_host(const cliproxy_host_api* host) { stored_host = *host; }

static int call_host(const char* method, const uint8_t* request, size_t len, cliproxy_buffer* response) {
    if (stored_host.call == NULL) {
        return 1;
    }
    return stored_host.call(stored_host.host_ctx, method, request, len, response);
}

static void free_host_buffer(void* ptr, size_t len) {
    if (stored_host.free_buffer != NULL && ptr != NULL) {
        stored_host.free_buffer(ptr, len);
    }
}
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unsafe"

	primer "github.com/Insulinocytus/cpa-quota-primer-plugin"
)

var instance = newRuntime(hostExecute, hostLog)

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if host == nil || host.abi_version != 1 || plugin == nil {
		return 1
	}
	C.store_host(host)
	plugin.abi_version = 1
	plugin.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)
	return 0
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, length C.size_t, response *C.cliproxy_buffer) C.int {
	if response == nil {
		return 1
	}
	response.ptr = nil
	response.len = 0
	if method == nil || (length > 0 && request == nil) {
		writeResponse(response, rpcFailure("invalid_rpc_request", "missing method or request"))
		return 1
	}
	var raw []byte
	if length > 0 {
		raw = C.GoBytes(unsafe.Pointer(request), C.int(length))
	}
	writeResponse(response, instance.handle(C.GoString(method), raw))
	return 0
}

//export cliproxyPluginFree
func cliproxyPluginFree(ptr unsafe.Pointer, _ C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
}

// The host unloads the library after shutdown returns, so background work
// must be joined first.
//
//export cliproxyPluginShutdown
func cliproxyPluginShutdown() { instance.stop() }

func writeResponse(response *C.cliproxy_buffer, raw []byte) {
	response.ptr = C.CBytes(raw)
	if response.ptr != nil {
		response.len = C.size_t(len(raw))
	}
}

type hostError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"http_status"`
}

func callHost(method string, payload any) (json.RawMessage, *hostError) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, &hostError{Code: "marshal_failed", Message: err.Error()}
	}
	cMethod := C.CString(method)
	defer C.free(unsafe.Pointer(cMethod))
	cRequest := C.CBytes(raw)
	defer C.free(cRequest)
	var response C.cliproxy_buffer
	code := C.call_host(cMethod, (*C.uint8_t)(cRequest), C.size_t(len(raw)), &response)
	var out []byte
	if response.ptr != nil {
		out = C.GoBytes(response.ptr, C.int(response.len))
		C.free_host_buffer(response.ptr, response.len)
	}
	var envelope struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		Error  *hostError      `json:"error"`
	}
	if len(out) == 0 || json.Unmarshal(out, &envelope) != nil {
		return nil, &hostError{Code: "host_call_failed", Message: fmt.Sprintf("%s returned no envelope (code %d)", method, int(code))}
	}
	if !envelope.OK {
		if envelope.Error == nil {
			return nil, &hostError{Code: "host_call_failed", Message: method + " failed"}
		}
		return nil, envelope.Error
	}
	return envelope.Result, nil
}

// hostExecute sends the request through host.model.execute, the same
// executor path (headers, token refresh, proxy, request log) as user traffic.
//
// The host runs this callback under context.Background() and offers no
// cancel for it, so when ctx is done the call is detached: it finishes inside
// the host and its result is dropped. This lets shutdown and reconfigure
// return instead of waiting on an upstream that may never answer. On return
// the detached call only touches Go code and the copied host table; the
// library stays mapped (Windows never unloads it, ELF c-shared is linked
// -z nodelete).
func hostExecute(ctx context.Context, req primer.ModelRequest) (primer.ModelResponse, error) {
	type outcome struct {
		resp primer.ModelResponse
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		resp, err := executeOnHost(req)
		done <- outcome{resp, err}
	}()
	select {
	case o := <-done:
		return o.resp, o.err
	case <-ctx.Done():
		return primer.ModelResponse{}, ctx.Err()
	}
}

func executeOnHost(req primer.ModelRequest) (primer.ModelResponse, error) {
	result, failure := callHost("host.model.execute", req)
	if failure != nil {
		return primer.ModelResponse{StatusCode: failure.HTTPStatus}, errors.New(failure.Code + ": " + failure.Message)
	}
	var resp primer.ModelResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return resp, fmt.Errorf("decode host.model.execute result: %w", err)
	}
	return resp, nil
}

// hostLog renders fields into the message: the host's text formatter prints
// only its own whitelist of field keys and would drop account, trigger, raw.
func hostLog(level, message string, fields map[string]any) {
	var b strings.Builder
	b.WriteString(message)
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		fmt.Fprintf(&b, " %s=%q", key, fmt.Sprint(fields[key]))
	}
	callHost("host.log", map[string]any{"level": level, "message": b.String(), "fields": map[string]any{"plugin_id": primer.PluginID}})
}
