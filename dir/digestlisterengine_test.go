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
	"github.com/wking/casengine"
	"golang.org/x/net/context"
)

func TestDigestListerEngineGood(t *testing.T) {
	ctx := context.Background()

	temp, err := ioutil.TempDir("", "casengine-dir-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(temp)

	pattern := `^.*/blobs/(?P<algorithm>[a-z0-9+._-]+)/[a-zA-Z0-9=_-]{1,2}/(?P<encoded>[a-zA-Z0-9=_-]{1,})$`
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

	if filepath.Separator != '/' {
		t.Fatalf("full URI not implemented for filepath.Separator %q", filepath.Separator)
	}

	engine, err := NewDigestListerEngine(
		ctx,
		temp,
		fmt.Sprintf("file://%s/blobs/{algorithm}/{encoded:2}/{encoded}", temp),
		(&RegexpGetDigest{
			Regexp: getDigestRegexp,
		}).GetDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(ctx)

	runPut(ctx, t, engine, temp)
	runGet(ctx, t, engine)
	runAlgorithms(ctx, t, engine)
	runDigests(ctx, t, engine)
	runDelete(ctx, t, engine)
}

func runDigests(ctx context.Context, t *testing.T, engine casengine.DigestLister) {
	t.Run("digests", func(t *testing.T) {
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
}
