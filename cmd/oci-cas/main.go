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

package main

import (
	_ "crypto/sha256"
	_ "crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/wking/casengine"
	_ "github.com/wking/casengine/template"
	"github.com/xiekeyang/oci-discovery/tools/engine"
	"golang.org/x/net/context"
)

func main() {
	app := cli.NewApp()
	app.Name = "oci-cas"
	app.Version = "0.1.0"
	app.Usage = "Open Container Intiative Content Addressable Storage"
	app.ArgsUsage = "DIGEST..."

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "log-level",
			Value: "error",
			Usage: "Log level (panic, fatal, error, warn, info, or debug)",
		},
		cli.StringFlag{
			Name:  "file",
			Usage: "Effective root for file URIs.  To allow access to your entire filesystem, use '--file /'.  More restricted values are recommended to avoid accessing sensitive information.  The default is to disable file URIs entirely; you must set this flag to enable them.",
		},
	}

	app.Before = func(c *cli.Context) (err error) {
		logLevelString := c.GlobalString("log-level")
		logLevel, err := logrus.ParseLevel(logLevelString)
		if err != nil {
			logrus.Fatal(err)
		}
		logrus.SetLevel(logLevel)

		return nil
	}

	app.Action = func(c *cli.Context) (err error) {
		ctx := context.Background()

		if c.GlobalIsSet("file") {
			path := c.GlobalString("file")
			transport := http.NewFileTransport(http.Dir(path))
			http.DefaultTransport.(*http.Transport).RegisterProtocol("file", transport)
		}

		var configReferences []engine.Reference
		err = json.NewDecoder(os.Stdin).Decode(&configReferences)
		if err != nil {
			logrus.Error("failed to read engine config from stdin")
			return err
		}

		engines := []casengine.Engine{}
		for _, configReference := range configReferences {
			constructor, ok := casengine.Constructors[configReference.Config.Protocol]
			if !ok {
				logrus.Debugf("unsupported CAS-engine protocol %q (%v)", configReference.Config.Protocol, casengine.Constructors)
				continue
			}

			eng, err := constructor(ctx, configReference.URI, configReference.Config.Data)
			if err != nil {
				logrus.Warnf("failed to initialize %s CAS engine with %v: %s", configReference.Config.Protocol, configReference.Config.Data, err)
				continue
			}
			defer eng.Close(ctx)

			engines = append(engines, eng)
		}
		if len(engines) == 0 {
			return fmt.Errorf("failed to load any engine configurations")
		}

	DigestLoop:
		for _, digestString := range c.Args() {
			digest, err := digest.Parse(digestString)
			if err != nil {
				logrus.Errorf("failed to parse digest %s", digestString)
				return err
			}

			logrus.Debugf("getting %s with %v", digest, engines)
			for _, eng := range engines {
				logrus.Debugf("checking engine %v", eng)
				rawReader, err := eng.Get(ctx, digest)
				if err != nil {
					logrus.Warnf("failed to get %s: %s", digest, err)
					continue
				}
				verifier := digest.Verifier()
				verifiedReader := io.TeeReader(rawReader, verifier)
				bytes, err := ioutil.ReadAll(verifiedReader)
				if !verifier.Verified() {
					logrus.Warnf("invalid bytes for %s", digest)
					continue
				}
				_, err = os.Stdout.Write(bytes)
				if err != nil {
					return err
				}
				continue DigestLoop
			}
			return fmt.Errorf("failed to retrieve %s", digest)
		}

		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		logrus.Fatal(err)
	}
}
