/*
 * Minio Client (C) 2014, 2015 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"encoding/json"

	"github.com/fatih/color"
	"github.com/minio/cli"
	"github.com/minio/mc/pkg/console"
	"github.com/minio/minio/pkg/probe"
)

var (
	mbFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "region",
			Value: "us-east-1",
			Usage: "Specify bucket region. Defaults to ‘us-east-1’.",
		},
	}
)

// make a bucket or folder.
var mbCmd = cli.Command{
	Name:   "mb",
	Usage:  "Make a bucket or a folder.",
	Action: mainMakeBucket,
	Flags:  append(mbFlags, globalFlags...),
	CustomHelpTemplate: `NAME:
   mc {{.Name}} - {{.Usage}}

USAGE:
   mc {{.Name}} [FLAGS] TARGET [TARGET...]

FLAGS:
  {{range .Flags}}{{.}}
  {{end}}
EXAMPLES:
   1. Create a bucket on Amazon S3 cloud storage.
      $ mc {{.Name}} s3/mynewbucket

   2. Create a new bucket on Google Cloud Storage.
      $ mc {{.Name}} gcs/miniocloud

   4. Create a new bucket on Amazon S3 cloud storage in region ‘us-west-2’.
      $ mc {{.Name}} --region=us-west-2 s3/myregionbucket

   5. Create a new directory including its missing parents (equivalent to ‘mkdir -p’).
      $ mc {{.Name}} /tmp/this/new/dir1

   6. Create multiple directories including its missing parents (behavior similar to ‘mkdir -p’).
      $ mc {{.Name}} /mnt/sdb/mydisk /mnt/sdc/mydisk /mnt/sdd/mydisk
`,
}

// makeBucketMessage is container for make bucket success and failure messages.
type makeBucketMessage struct {
	Status string `json:"status"`
	Bucket string `json:"bucket"`
	Region string `json:"region"`
}

// String colorized make bucket message.
func (s makeBucketMessage) String() string {
	return console.Colorize("MakeBucket", "Bucket created successfully ‘"+s.Bucket+"’.")
}

// JSON jsonified make bucket message.
func (s makeBucketMessage) JSON() string {
	makeBucketJSONBytes, e := json.Marshal(s)
	fatalIf(probe.NewError(e), "Unable to marshal into JSON.")

	return string(makeBucketJSONBytes)
}

// Validate command line arguments.
func checkMakeBucketSyntax(ctx *cli.Context) {
	if !ctx.Args().Present() {
		cli.ShowCommandHelpAndExit(ctx, "mb", 1) // last argument is exit code
	}
}

// mainMakeBucket is entry point for mb command.
func mainMakeBucket(ctx *cli.Context) error {
	// Set global flags from context.
	setGlobalsFromContext(ctx)

	// check 'mb' cli arguments.
	checkMakeBucketSyntax(ctx)

	// Additional command speific theme customization.
	console.SetColor("MakeBucket", color.New(color.FgGreen, color.Bold))

	// Save region.
	region := ctx.String("region")

	var cErr error
	for i := range ctx.Args() {
		targetURL := ctx.Args().Get(i)
		// Instantiate client for URL.
		clnt, err := newClient(targetURL)
		if err != nil {
			errorIf(err.Trace(targetURL), "Invalid target ‘"+targetURL+"’.")
			cErr = exitStatus(globalErrorExitStatus)
			continue
		}

		// Make bucket.
		err = clnt.MakeBucket(region)
		if err != nil {
			errorIf(err.Trace(targetURL), "Unable to make bucket ‘"+targetURL+"’.")
			cErr = exitStatus(globalErrorExitStatus)
			continue
		}

		// Successfully created a bucket.
		printMsg(makeBucketMessage{Status: "success", Bucket: targetURL})
	}
	return cErr
}
