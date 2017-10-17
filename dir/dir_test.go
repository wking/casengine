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

package dir

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestGood(t *testing.T) {
	ctx := context.Background()

	temp, err := ioutil.TempDir("", "casengine-dir-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(temp)

	pattern := `^blobs/(?P<algorithm>[a-z0-9+._-]+)/[a-zA-Z0-9=_-]{1,2}/(?P<encoded>[a-zA-Z0-9=_-]{1,})$`
	if filepath.Separator != '/' {
		if filepath.Separator == '\\' {
			pattern = strings.Replace(pattern, "/", `\\`, -1)
		} else {
			t.Fatalf("unknown path separator %q", string(filepath.Separator))
		}
	}
	getDigestRegexp, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatal(err)
	}

	engine, err := New(
		ctx,
		temp,
		"blobs/{algorithm}/{encoded:2}/{encoded}",
		func(path string) (dig digest.Digest, err error) {
			matches := make(map[string]string)
			submatches := getDigestRegexp.FindStringSubmatch(path)
			for i, submatchName := range getDigestRegexp.SubexpNames() {
				if submatchName == "" {
					continue
				}
				if i > len(submatches) {
					return "", fmt.Errorf("%q does not match %q", path, pattern)
				}
				matches[submatchName] = submatches[i]
			}

			algorithm, ok := matches["algorithm"]
			if !ok {
				return "", fmt.Errorf("no 'algorithm' capturing group in %q", pattern)
			}

			encoded, ok := matches["encoded"]
			if !ok {
				return "", fmt.Errorf("no 'encoded' capturing group in %q", pattern)
			}

			return digest.Parse(fmt.Sprintf("%s:%s", algorithm, encoded))
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(ctx)

	dirEngine, ok := engine.(*Engine)
	if !ok {
		t.Fatalf("template.New() did not return a *template.Engine")
	}

	var digestSha256 digest.Digest
	bodyIn := "Hello, World!"
	t.Run("put default algorithm", func(t *testing.T) {
		digestSha256, err = engine.Put(ctx, strings.NewReader(bodyIn))
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(
			t,
			"sha256:dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f",
			digestSha256.String(),
		)
	})

	var digestSha512 digest.Digest
	t.Run("put SHA-512", func(t *testing.T) {
		dirEngine.Algorithm = digest.SHA512
		defer func() {
			dirEngine.Algorithm = digest.SHA256
		}()

		digestSha512, err = engine.Put(ctx, strings.NewReader(bodyIn))
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(
			t,
			"sha512:374d794a95cdcfd8b35993185fef9ba368f160d8daf432d08ba9f1ed1e5abe6cc69291e0fa2fe0006a52570ef18c19def4e617c33ce52ef0a6e5fbe318cb0387",
			digestSha512.String(),
		)
	})

	t.Run("get", func(t *testing.T) {
		reader, err := engine.Get(ctx, digestSha256)
		if err != nil {
			t.Fatal(err)
		}

		bodyOut, err := ioutil.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, bodyIn, string(bodyOut))
	})

	t.Run("expected path location", func(t *testing.T) {
		path := filepath.Join(temp, "blobs", "sha256", "df", "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f")
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}

		bodyOut, err := ioutil.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, bodyIn, string(bodyOut))
	})

	t.Run("algorithms", func(t *testing.T) {
		for _, testcase := range []struct {
			prefix   string
			size     int
			from     int
			expected []string
		}{
			{
				prefix:   "",
				size:     0,
				from:     0,
				expected: []string{},
			},
			{
				prefix:   "",
				size:     -1,
				from:     0,
				expected: []string{"sha256", "sha384", "sha512"},
			},
			{
				prefix:   "",
				size:     1,
				from:     0,
				expected: []string{"sha256"},
			},
			{
				prefix:   "",
				size:     2,
				from:     1,
				expected: []string{"sha384", "sha512"},
			},
			{
				prefix:   "sha5",
				size:     -1,
				from:     0,
				expected: []string{"sha512"},
			},
		} {
			name := fmt.Sprintf("%q,%d,%d", testcase.prefix, testcase.size, testcase.from)
			t.Run(name, func(t *testing.T) {
				algorithms := []string{}
				err := engine.Algorithms(
					ctx,
					testcase.prefix,
					testcase.size,
					testcase.from,
					func(ctx context.Context, algorithm digest.Algorithm) (err error) {
						algorithms = append(algorithms, algorithm.String())
						return nil
					},
				)
				if err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, testcase.expected, algorithms)
			})
		}
	})

	t.Run("digests", func(t *testing.T) {
		// Inject sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
		_, err = engine.Put(ctx, strings.NewReader(""))
		if err != nil {
			t.Fatal(err)
		}

		for _, testcase := range []struct {
			algorithm digest.Algorithm
			prefix    string
			size      int
			from      int
			expected  []string
		}{
			{
				algorithm: "",
				prefix:    "",
				size:      0,
				from:      0,
				expected:  []string{},
			},
			{
				algorithm: "",
				prefix:    "",
				size:      -1,
				from:      0,
				expected: []string{
					"sha256:dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f",
					"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					"sha512:374d794a95cdcfd8b35993185fef9ba368f160d8daf432d08ba9f1ed1e5abe6cc69291e0fa2fe0006a52570ef18c19def4e617c33ce52ef0a6e5fbe318cb0387",
				},
			},
			{
				algorithm: digest.SHA256,
				prefix:    "",
				size:      -1,
				from:      0,
				expected: []string{
					"sha256:dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f",
					"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				},
			},
			{
				algorithm: digest.SHA256,
				prefix:    "e",
				size:      -1,
				from:      0,
				expected: []string{
					"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				},
			},
			{
				algorithm: "",
				prefix:    "",
				size:      2,
				from:      0,
				expected: []string{
					"sha256:dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f",
					"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				},
			},
		} {
			name := fmt.Sprintf("%q,%q,%d,%d", testcase.algorithm, testcase.prefix, testcase.size, testcase.from)
			t.Run(name, func(t *testing.T) {
				digests := []string{}
				err := engine.Digests(
					ctx,
					testcase.algorithm,
					testcase.prefix,
					testcase.size,
					testcase.from,
					func(ctx context.Context, digest digest.Digest) (err error) {
						digests = append(digests, digest.String())
						return nil
					},
				)
				if err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, testcase.expected, digests)
			})
		}
	})

	t.Run("delete", func(t *testing.T) {
		err = engine.Delete(ctx, digestSha256)
		if err != nil {
			t.Fatal(err)
		}

		_, err := engine.Get(ctx, digestSha256)
		if err == nil {
			t.Fatalf("Get() succeeded after deletion")
		}

		err = engine.Delete(ctx, digestSha256)
		if err != nil {
			t.Fatal(err)
		}
	})
}
