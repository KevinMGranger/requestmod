# requestmod

Easily modify golang http requests.

requestmod lets you wrap an existing http.RoundTripper, modifying each request that comes through.

It is almost entirely adapted from code in the [oauth2 package.](https://golang.org/x/oauth2)

# Example(s)

Override the User-Agent header on each request:

    cli := new(http.Client)
    cli.Transport = requestmod.NewTransport(nil, func(req *http.Request) error {
    		req.Header.Set("User-Agent", "MyAwesomeClient")
    		return nil
    })


Reject requests that go to a certain host:

    cli := new(http.Client)
    cli.Transport = requestmod.NewTransport(nil, func(req *http.Request) error {
    		if req.URL.Host == "ravenholm" {
    			return errors.New("We don't go there anymore.")
    		}
    	return nil
    })


# TODO / Bugs

- The tests are terrible... but at least they have 100% coverage.
