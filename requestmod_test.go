package requestmod

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type verifier func(*http.Request)

type errorTransport struct{}

func (*errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("Whoops")
}

func errorMod(mod *http.Request) error {
	return errors.New("Oh dear")
}

var expecting string = "this is a test"

func makeClient(Base http.RoundTripper, RequestVisitor RequestVisitor) *http.Client {
	cli := new(http.Client)
	trans := NewTransport(Base, RequestVisitor)
	cli.Transport = trans
	return cli
}

func TestRegularRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if head := r.Header.Get("X-Test-Header"); head != expecting {
			t.Errorf("Test header was incorrect: wanted %v, got %v", expecting, head)
			http.Error(w, "Where's your header?", 400)
		} else {
			io.WriteString(w, "Test succeeded")
		}
	}))
	defer ts.Close()

	cli := makeClient(nil, func(mod *http.Request) error {
		mod.Header.Set("X-Test-Header", expecting)
		return nil
	})

	url, _ := url.Parse(ts.URL)

	resp, _ := cli.Do(&http.Request{
		URL: url,
		Header: map[string][]string{
			"X-Test-Header-Copy": {"please"},
		},
	})

	ioutil.ReadAll(resp.Body)
	resp.Body.Close()
}

func TestModError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer ts.Close()

	cli := makeClient(nil, errorMod)

	_, err := cli.Get(ts.URL)

	if err == nil {
		t.Error("Did *not* get an expected modifier error.")
	}
}

func TestBaseError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer ts.Close()

	cli := makeClient(&errorTransport{}, nil)

	_, err := cli.Get(ts.URL)

	if err == nil {
		t.Error("Did *not* get an expected base error.")
	}
}

func TestCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer ts.Close()

	cli := makeClient(nil, nil)

	url, _ := url.Parse(ts.URL)

	req := http.Request{
		URL: url,
	}
	cli.Do(&req)
	type canceler interface {
		CancelRequest(*http.Request)
	}
	if cr, ok := cli.Transport.(canceler); ok {
		cr.CancelRequest(&req)
	}
	// TODO: how to verify it was actually cancelled?
}
