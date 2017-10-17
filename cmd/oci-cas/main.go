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
	"archive/zip"
	_ "crypto/sha256"
	_ "crypto/sha512"
	"fmt"
	"net/http"
	"os"

	"github.com/omeid/go-tarfs"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	_ "github.com/wking/casengine/read/template"
	"golang.org/x/tools/godoc/vfs/httpfs"
	"golang.org/x/tools/godoc/vfs/zipfs"
)

func main() {
	app := cli.NewApp()
	app.Name = "oci-cas"
	app.Version = "0.1.0"
	app.Usage = "Open Container Intiative Content Addressable Storage"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "log-level",
			Value: "error",
			Usage: "Log level (panic, fatal, error, warn, info, or debug)",
		},
		cli.StringFlag{
			Name:  "file",
			Usage: "Effective root for file URIs.  To allow access to your entire filesystem, use '--file /'.  More restricted values are recommended to avoid accessing sensitive information.  The default is to disable file URIs entirely; you must set this flag (or another --*-file flag) to enable them.",
		},
		cli.StringFlag{
			Name:  "tar-file",
			Usage: "Effective root for file URIs in a tape archive file (tarball).  As an alternative to --file, use the tape archive at this path as the root of the file URI filesystem.",
		},
		cli.StringFlag{
			Name:  "zip-file",
			Usage: "Effective root for file URIs in a zip archive file.  As an alternative to --file, use the zip archive at this path as the root of the file URI filesystem.",
		},
	}

	app.Commands = []cli.Command{
		get,
	}

	app.Before = func(c *cli.Context) (err error) {
		logLevelString := c.GlobalString("log-level")
		logLevel, err := logrus.ParseLevel(logLevelString)
		if err != nil {
			logrus.Fatal(err)
		}
		logrus.SetLevel(logLevel)
		logrus.Debugf("set log level to %s", logLevelString)

		if c.GlobalIsSet("file") {
			if c.GlobalIsSet("tar-file") {
				return fmt.Errorf("setting both --file and --tar-file is invalid")
			}
			if c.GlobalIsSet("zip-file") {
				return fmt.Errorf("setting both --file and --zip-file is invalid")
			}
			path := c.GlobalString("file")
			transport := http.NewFileTransport(http.Dir(path))
			http.DefaultTransport.(*http.Transport).RegisterProtocol("file", transport)
		}

		if c.GlobalIsSet("tar-file") {
			if c.GlobalIsSet("zip-file") {
				return fmt.Errorf("setting both --tar-file and --zip-file is invalid")
			}
			path := c.GlobalString("tar-file")
			reader, err := os.Open(path)
			if err != nil {
				return err
			}
			tarFS, err := tarfs.New(reader)
			if err != nil {
				err2 := reader.Close()
				if err2 != nil {
					logrus.Warn("failed to close the tar reader")
				}
				return err
			}
			err = reader.Close()
			if err != nil {
				return err
			}
			transport := http.NewFileTransport(tarFS)
			http.DefaultTransport.(*http.Transport).RegisterProtocol("file", transport)
		}

		if c.GlobalIsSet("zip-file") {
			path := c.GlobalString("zip-file")
			reader, err := zip.OpenReader(path)
			if err != nil {
				return err
			}
			transport := http.NewFileTransport(httpfs.New(zipfs.New(reader, path)))
			http.DefaultTransport.(*http.Transport).RegisterProtocol("file", transport)
		}

		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		logrus.Fatal(err)
	}
}
