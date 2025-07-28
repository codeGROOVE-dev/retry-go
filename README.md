# retry

[![Release](https://img.shields.io/github/release/codeGROOVE-dev/retry-go.svg?style=flat-square)](https://github.com/codeGROOVE-dev/retry-go/releases/latest)
[![Software License](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat-square)](LICENSE.md)
![GitHub Actions](https://github.com/codeGROOVE-dev/retry-go/actions/workflows/workflow.yaml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/codeGROOVE-dev/retry-go?style=flat-square)](https://goreportcard.com/report/github.com/codeGROOVE-dev/retry-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/codeGROOVE-dev/retry-go.svg)](https://pkg.go.dev/github.com/codeGROOVE-dev/retry-go)

## Fork Information

This is an actively maintained fork of [avast/retry-go](https://github.com/avast/retry-go), focused on reliability and simplicity. We extend our gratitude to the original authors and contributors at Avast for creating this excellent library.

This fork retains the original v4 API provided by the retry-go codebase, and is a drop-in replacement.

**Key improvements in this fork:**

- Zero dependencies (we removed usage of testify and it's dependencies)
- Enhanced reliability and edge case handling
- Active maintenance and bug fixes
- Adherance to Go best practices, namely:
  - https://go.dev/wiki/TestComments
  - https://go.dev/wiki/CodeReviewComments
  - https://go.dev/doc/effective_go

**Original Project:** [github.com/avast/retry-go](https://github.com/avast/retry-go)

# SYNOPSIS

HTTP GET with retry:

    url := "http://example.com"
    var body []byte

    err := retry.Do(
    	func() error {
    		resp, err := http.Get(url)
    		if err != nil {
    			return err
    		}
    		defer resp.Body.Close()
    		body, err = ioutil.ReadAll(resp.Body)
    		if err != nil {
    			return err
    		}
    		return nil
    	},
    )

    if err != nil {
    	// handle error
    }

    fmt.Println(string(body))

HTTP GET with retry with data:

    url := "http://example.com"

    body, err := retry.DoWithData(
    	func() ([]byte, error) {
    		resp, err := http.Get(url)
    		if err != nil {
    			return nil, err
    		}
    		defer resp.Body.Close()
    		body, err := ioutil.ReadAll(resp.Body)
    		if err != nil {
    			return nil, err
    		}

    		return body, nil
    	},
    )

    if err != nil {
    	// handle error
    }

    fmt.Println(string(body))

[More examples](https://github.com/codeGROOVE-dev/retry-go/tree/master/examples)

# SEE ALSO

* [giantswarm/retry-go](https://github.com/giantswarm/retry-go) - slightly
complicated interface.

* [sethgrid/pester](https://github.com/sethgrid/pester) - only http retry for
http calls with retries and backoff

* [cenkalti/backoff](https://github.com/cenkalti/backoff) - Go port of the
exponential backoff algorithm from Google's HTTP Client Library for Java. Really
complicated interface.

* [rafaeljesus/retry-go](https://github.com/rafaeljesus/retry-go) - looks good,
slightly similar as this package, don't have 'simple' `Retry` method

* [matryer/try](https://github.com/matryer/try) - very popular package,
nonintuitive interface (for me)

## Contributing

Contributions are very much welcome.

### Makefile

Makefile provides several handy rules, like README.md `generator` , `setup` for prepare build/dev environment, `test`, `cover`, etc...

Try `make help` for more information.

### Before pull request

> maybe you need `make setup` in order to setup environment

please try:
* run tests (`make test`)
* run linter (`make lint`)
* if your IDE don't automaticaly do `go fmt`, run `go fmt` (`make fmt`)
