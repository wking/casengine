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

// Package dir implements a directory-based CAS engine.
package dir

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/wking/casengine"
	"github.com/wking/casengine/read/template"
	"golang.org/x/net/context"
)

// GetDigest calculates the digest corresponding to a given relative
// path.  This is effectively the inverse of URI Template expansion,
// and is required to support Digests.
type GetDigest func(path string) (digest digest.Digest, err error)

// RegexpGetDigest is a helper structure for regular-expression based
// GetDigest implementations.
type RegexpGetDigest struct {
	Regexp *regexp.Regexp
}

// Engine is a CAS engine based on the local filesystem.
type Engine struct {
	path      string
	temp      string
	reader    *template.Engine
	getDigest GetDigest

	// Algorithm selects the Algorithm used for Put.
	Algorithm digest.Algorithm
}

// GetDigest implements GetDigest for RegexpGetDigest.
func (r *RegexpGetDigest) GetDigest(path string) (dig digest.Digest, err error) {
	matches := make(map[string]string)
	submatches := r.Regexp.FindStringSubmatch(path)
	for i, submatchName := range r.Regexp.SubexpNames() {
		if submatchName == "" {
			continue
		}
		if i > len(submatches) {
			return "", fmt.Errorf("%q does not match %q", path, r.Regexp.String())
		}
		matches[submatchName] = submatches[i]
	}

	algorithm, ok := matches["algorithm"]
	if !ok {
		return "", fmt.Errorf("no 'algorithm' capturing group in %q", r.Regexp.String())
	}

	encoded, ok := matches["encoded"]
	if !ok {
		return "", fmt.Errorf("no 'encoded' capturing group in %q", r.Regexp.String())
	}

	return digest.Parse(fmt.Sprintf("%s:%s", algorithm, encoded))
}

// New creates a new CAS-engine instance.
func New(ctx context.Context, path string, uri string, getDigest GetDigest) (engine casengine.Engine, err error) {
	temp, err := ioutil.TempDir(path, ".casengine-")
	if err != nil {
		return nil, err
	}

	base, err := url.Parse("file:///")
	if err != nil {
		return nil, err
	}

	config := map[string]string{
		"uri": uri,
	}

	reader, err := template.New(ctx, base, config)
	if err != nil {
		return nil, err
	}

	readEngine, ok := reader.(*template.Engine)
	if !ok {
		return nil, fmt.Errorf("template.New() did not return a *template.Engine")
	}

	readEngine.Client = &http.Client{
		Transport: http.NewFileTransport(http.Dir(path)),
	}

	return &Engine{
		path:      path,
		temp:      temp,
		reader:    readEngine,
		getDigest: getDigest,
		Algorithm: digest.SHA256,
	}, nil
}

// Get implements Reader.Get.
func (engine *Engine) Get(ctx context.Context, digest digest.Digest) (reader io.ReadCloser, err error) {
	return engine.reader.Get(ctx, digest)
}

// Algorithms implements AlgorithmLister.Algorithms.
func (engine *Engine) Algorithms(ctx context.Context, prefix string, size int, from int, callback casengine.AlgorithmCallback) (err error) {
	if size == 0 {
		return nil
	}
	offset := 0
	count := 0
	for _, algorithm := range []digest.Algorithm{
		digest.SHA256,
		digest.SHA384,
		digest.SHA512,
	} {
		if prefix == "" || strings.HasPrefix(algorithm.String(), prefix) {
			if offset >= from {
				err = callback(ctx, algorithm)
				if err != nil {
					return err
				}
				count++
				if size != -1 && count >= size {
					return nil
				}
			}
			offset++
		}
	}
	return nil
}

// Digests implements DigestLister.Digests.
func (engine *Engine) Digests(ctx context.Context, algorithm digest.Algorithm, prefix string, size int, from int, callback casengine.DigestCallback) (err error) {
	if size == 0 {
		return nil
	}
	globAlgorithm := algorithm.String()
	if globAlgorithm == "" {
		globAlgorithm = "*"
	}
	globDigest := digest.Digest(fmt.Sprintf("%s:*", globAlgorithm))
	glob, err := engine.getPath(globDigest)
	if err != nil {
		return err
	}

	matches, err := filepath.Glob(glob)
	if err != nil {
		return err
	}

	offset := 0
	count := 0
	for _, match := range matches {
		rel, err := filepath.Rel(engine.path, match)
		if err != nil {
			logrus.Warnf("cannot compute relative digest path %q (%s)", match, err)
			continue
		}

		digest, err := engine.getDigest(rel)
		if err != nil {
			logrus.Warnf("cannot compute digest for %q (%s)", rel, err)
			continue
		}

		if algorithm.String() == "" || digest.Algorithm() == algorithm {
			if prefix == "" || strings.HasPrefix(digest.Encoded(), prefix) {
				if offset >= from {
					err = callback(ctx, digest)
					if err != nil {
						return err
					}
					count++
					if size != -1 && count >= size {
						return nil
					}
				}
				offset++
			}
		}
	}
	return nil
}

// Put implements Writer.Put.
func (engine *Engine) Put(ctx context.Context, algorithm digest.Algorithm, reader io.Reader) (dig digest.Digest, err error) {
	if algorithm.String() == "" {
		algorithm = engine.Algorithm
	}
	digester := algorithm.Digester()

	file, err := ioutil.TempFile(engine.temp, "blob-")
	if err != nil {
		return "", err
	}

	defer func() {
		if err != nil {
			err2 := os.Remove(file.Name())
			if err2 != nil {
				logrus.Error(err2)
			}
		}
	}()

	hashingWriter := io.MultiWriter(file, digester.Hash())
	_, err = io.Copy(hashingWriter, reader)
	if err != nil {
		return "", err
	}
	file.Close()

	dig = digester.Digest()
	path, err := engine.getPath(dig)
	if err != nil {
		return "", err
	}

	err = os.MkdirAll(filepath.Dir(path), 0777)
	if err != nil {
		return "", err
	}

	err = os.Rename(file.Name(), path)
	if err != nil {
		return "", err
	}

	return dig, nil
}

// Delete implements Deleter.Delete.
func (engine *Engine) Delete(ctx context.Context, digest digest.Digest) (err error) {
	path, err := engine.getPath(digest)
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Close implements Closer.Close.
func (engine *Engine) Close(ctx context.Context) (err error) {
	err = os.RemoveAll(engine.temp)
	if err != nil {
		return err
	}

	return engine.reader.Close(ctx)
}

func (engine *Engine) getPath(digest digest.Digest) (path string, err error) {
	if filepath.Separator != '/' {
		return "", fmt.Errorf("getPath not implemented for filepath.Separator %q", filepath.Separator)
	}

	uri, err := engine.reader.URI(digest)
	if err != nil {
		return "", err
	}

	if uri.Scheme != "file" || uri.Opaque != "" || uri.User != nil || uri.Host != "" || uri.RawQuery != "" || uri.Fragment != "" {
		return "", fmt.Errorf("invalid URI: %q", uri)
	}

	return filepath.Join(engine.path, strings.TrimLeft(uri.Path, "/")), nil
}
