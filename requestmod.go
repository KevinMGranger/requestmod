// Package requestmod offers an http.RoundTripper that will modify and/or inspect Requests.
//
// It is mostly adapted from the transport in the oauth2 package (https://golang.org/x/oauth2).
package requestmod

import (
	"io"
	"net/http"
	"sync"
)

// A RequestVisitor is given a HTTP request, optionally returning an error.
// The function is allowed to modify the request, as it will always be given a new copy.
// It must be safe to visit from multiple goroutines.
type RequestVisitor func(req *http.Request) error

// A Transport wraps an existing http.RoundTripper, using the given RequestVisitor
// on each request.
type Transport struct {
	// Base is the wrapped RoundTripper.
	// It must not be nil.
	// You should not modify this field: it is only exported so that you may access
	// specific methods of the underlying RoundTripper.
	Base http.RoundTripper

	// RequestVisitor is called for each request.
	// If nil, the request is sent untouched.
	RequestVisitor RequestVisitor

	mu sync.Mutex // for modReq

	// modReq maps the original http.Request to the modified one, because
	// RoundTrippers are not allowed to modify the original, yet we need to keep track of it.
	modReq map[*http.Request]*http.Request
}

// NewTransport creates a Transport with the given RoundTripper and RequestVisitor.
// If Base is nil, http.DefaultTransport is used instead.
func NewTransport(Base http.RoundTripper, RequestVisitor RequestVisitor) http.RoundTripper {
	if Base == nil {
		Base = http.DefaultTransport
	}
	return &Transport{
		Base:           Base,
		RequestVisitor: RequestVisitor,
		modReq:         make(map[*http.Request]*http.Request),
	}
}

// RoundTrip implements the RoundTripper interface.
// It will apply each modifier to the request.
// It will return an error if any modifier returned an error.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	mod := cloneRequest(req)

	if t.RequestVisitor != nil {
		err := t.RequestVisitor(mod)
		if err != nil {
			return nil, err
		}
	}

	t.setModReq(req, mod)
	res, err := t.Base.RoundTrip(mod)

	if err != nil {
		t.setModReq(req, nil)
		return nil, err
	}
	res.Body = &onEOFReader{
		rc: res.Body,
		fn: func() { t.setModReq(req, nil) },
	}
	return res, nil
}

// CancelRequest cancels an in-flight request by closing its connection.
// This will only work if the base transport supports canceling requests.
func (t *Transport) CancelRequest(req *http.Request) {
	type canceler interface {
		CancelRequest(*http.Request)
	}
	if cr, ok := t.Base.(canceler); ok {
		t.mu.Lock()
		modReq := t.modReq[req]
		delete(t.modReq, req)
		t.mu.Unlock()
		cr.CancelRequest(modReq)
	}
}

// setModReq updates the map mapping original requests to their modified versions.
func (t *Transport) setModReq(orig, mod *http.Request) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if mod == nil {
		delete(t.modReq, orig)
	} else {
		t.modReq[orig] = mod
	}
}

// cloneRequest creates a clone of the given request, copying over header values.
func cloneRequest(orig *http.Request) *http.Request {
	mod := new(http.Request)
	*mod = *orig
	mod.Header = make(http.Header, len(orig.Header))
	for k, s := range orig.Header {
		mod.Header[k] = append([]string(nil), s...)
	}
	return mod
}

// An onEOFReader calls a given function when the wrapped ReadCloser returns EOF.
// The func will only be called once, even if EOF is hit somewhere, and then Close is called.
type onEOFReader struct {
	rc io.ReadCloser
	fn func()
}

func (r *onEOFReader) Read(p []byte) (n int, err error) {
	n, err = r.rc.Read(p)
	if err == io.EOF {
		r.runFunc()
	}
	return
}

func (r *onEOFReader) Close() error {
	err := r.rc.Close()
	r.runFunc()
	return err
}

func (r *onEOFReader) runFunc() {
	if fn := r.fn; fn != nil {
		fn()
		r.fn = nil
	}
}
