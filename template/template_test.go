// Copyright 2017 casengine contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package template

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/wking/casengine"
	"github.com/xiekeyang/oci-discovery/tools/engine"
	"golang.org/x/net/context"
)

func TestRegistration(t *testing.T) {
	_, ok := casengine.Constructors["oci-cas-template-v1"]
	if !ok {
		t.Fatalf("failed to register oci-cas-template-v1")
	}
}

func TestNewFromEngineConfigGood(t *testing.T) {
	ctx := context.Background()
	config := engine.Config{
		Data: map[string]interface{}{
			"uri": "a/b",
		},
	}
	base, err := url.Parse("https://example.com")
	if err != nil {
		t.Fatal(err)
	}

	engine, err := New(ctx, base, config.Data)
	if err != nil {
		t.Fatal(err)
	}

	err = engine.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewFromConfigBad(t *testing.T) {
	ctx := context.Background()
	base, err := url.Parse("https://example.com")
	if err != nil {
		t.Fatal(err)
	}

	for _, testcase := range []struct {
		name     string
		config   interface{}
		expected string
	}{
		{
			name:     "config not a map",
			config:   "not a map",
			expected: `CAS-template config is not a map\[string\]string: .*`,
		},
		{
			name:     "string->string config missing 'uri' property",
			config:   map[string]string{},
			expected: `CAS-template config missing required 'uri' property: .*`,
		},
		{
			name:     "string->interface config missing 'uri' property",
			config:   map[string]interface{}{},
			expected: `CAS-template config missing required 'uri' property: .*`,
		},
		{
			name: "uri not a string",
			config: map[string]interface{}{
				"uri": 1,
			},
			expected: `CAS-template config 'uri' is not a string: .*`,
		},
		{
			name: "uri string not a URI Template",
			config: map[string]string{
				"uri": "{",
			},
			expected: `malformed template`,
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			_, err := New(ctx, base, testcase.config)
			if err == nil {
				t.Fatalf("expected %s", testcase.expected)
			}
			assert.Regexp(t, testcase.expected, err.Error())
		})
	}
}

func TestGetPreFetchGood(t *testing.T) {
	ctx := context.Background()
	digest, err := digest.Parse("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	if err != nil {
		t.Fatal(err)
	}
	for _, testcase := range []struct {
		template string
		base     string
		expected string
	}{
		{
			template: "blob",
			base:     "https://example.com/a",
			expected: "https://example.com/blob",
		},
		{
			template: "blob",
			base:     "https://example.com/a/",
			expected: "https://example.com/a/blob",
		},
		{
			template: "https://example.com/{algorithm}/{encoded}",
			base:     "https://a.example.com/b/",
			expected: "https://example.com/sha256/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			template: "//example.com/{algorithm}/{encoded}",
			base:     "https://a.example.com/b/",
			expected: "https://example.com/sha256/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			template: "/{algorithm}/{encoded}",
			base:     "https://a.example.com/b/",
			expected: "https://a.example.com/sha256/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			template: "{algorithm}/{encoded}",
			base:     "https://a.example.com/b/",
			expected: "https://a.example.com/b/sha256/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			template: "{digest}",
			base:     "https://example.com/a/",
			expected: "https://example.com/a/sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			template: "{algorithm}/{encoded:2}/{encoded}",
			base:     "https://a.example.com/b/",
			expected: "https://a.example.com/b/sha256/e3/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	} {
		name := fmt.Sprintf("%s from %s", testcase.template, testcase.base)
		t.Run(name, func(t *testing.T) {
			base, err := url.Parse(testcase.base)
			if err != nil {
				t.Fatal(err)
			}

			config := map[string]string{
				"uri": testcase.template,
			}

			engine, err := New(ctx, base, config)
			if err != nil {
				t.Fatal(err)
			}
			defer engine.Close(ctx)

			request, err := engine.(*Engine).getPreFetch(digest)
			if err != nil {
				t.Fatal(err)
			}

			uri, err := url.Parse(testcase.expected)
			if err != nil {
				t.Fatal(err)
			}

			expected := &http.Request{
				Method: "GET",
			}

			assert.Equal(t, uri.String(), request.URL.String())
			request.URL = nil
			assert.Equal(t, expected, request)
		})
	}
}

func TestGetPreFetchBad(t *testing.T) {
	ctx := context.Background()
	config := map[string]string{
		"uri": "{+digest}",
	}

	engine, err := New(ctx, nil, config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(ctx)

	for _, testcase := range []struct {
		name     string
		digest   digest.Digest
		expected string
	}{
		{
			name:     "no scheme",
			digest:   ":",
			expected: "parse :: missing protocol scheme",
		},
		//{
		//	name:     "relative reference with unanchored engine",
		//	digest:   "blob",
		//	expected: "FIXME panic https://github.com/golang/go/issues/22229",
		//},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			request, err := engine.(*Engine).getPreFetch(testcase.digest)
			if err == nil {
				t.Fatalf("returned %s and did not raise the expected error", request.URL)
			}
			assert.Regexp(t, testcase.expected, err.Error())
		})
	}
}

func TestGetPostFetchGood(t *testing.T) {
	ctx := context.Background()
	config := map[string]string{
		"uri": "https://example.com/cas",
	}

	uri, err := url.Parse(config["uri"])
	if err != nil {
		t.Fatal(err)
	}

	request := &http.Request{
		URL: uri,
	}

	engine, err := New(ctx, uri, config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(ctx)

	for _, testcase := range []struct {
		digest digest.Digest
		status int
		body   string
	}{
		{
			digest: "sha256:dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f",
			status: http.StatusOK,
			body:   "Hello, World!",
		},
		{
			digest: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			status: http.StatusNoContent,
			body:   "",
		},
	} {
		t.Run(string(testcase.status), func(t *testing.T) {
			response := &http.Response{
				StatusCode: testcase.status,
				Request:    request,
				Body:       ioutil.NopCloser(strings.NewReader(testcase.body)),
			}

			reader, err := engine.(*Engine).getPostFetch(response, testcase.digest)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()

			body, err := ioutil.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, testcase.body, string(body))
		})
	}
}

func TestGetPostFetchBad(t *testing.T) {
	ctx := context.Background()
	digest, err := digest.Parse("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	if err != nil {
		t.Fatal(err)
	}

	config := map[string]string{
		"uri": "https://example.com/blob",
	}

	engine, err := New(ctx, nil, config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(ctx)

	uri, err := url.Parse(config["uri"])
	if err != nil {
		t.Fatal(err)
	}
	request := &http.Request{
		URL: uri,
	}

	for _, testcase := range []struct {
		label    string
		status   string
		body     string
		expected string
	}{
		{
			label:    "blob not found",
			status:   "404 Not Found",
			body:     "",
			expected: `file does not exist`,
		},
		{
			label:    "server error",
			status:   "500 Internal Server Error",
			body:     "",
			expected: `requested https://example.com/blob but got 500 Internal Server Error`,
		},
	} {
		t.Run(testcase.label, func(t *testing.T) {
			statusString := strings.SplitN(testcase.status, " ", 2)[0]
			status, err := strconv.Atoi(statusString)
			if err != nil {
				t.Fatal(err)
			}

			response := &http.Response{
				Status:     testcase.status,
				StatusCode: status,
				Request:    request,
				Body:       ioutil.NopCloser(strings.NewReader(testcase.body)),
			}

			reader, err := engine.(*Engine).getPostFetch(response, digest)
			if err == nil {
				body, err := ioutil.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				t.Fatalf("returned %s and did not raise the expected error", body)
			}

			assert.Regexp(t, testcase.expected, err.Error())
		})
	}
}
