/*
 * Minio Client (C) 2015, 2016 Minio, Inc.
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
	"fmt"
	"strings"

	"github.com/minio/cli"
)

//
//   * MIRROR ARGS - VALID CASES
//   =========================
//   mirror(d1..., d2) -> []mirror(d1/f, d2/d1/f)

// checkMirrorSyntax(URLs []string)
func checkMirrorSyntax(ctx *cli.Context) {
	if len(ctx.Args()) != 2 {
		cli.ShowCommandHelpAndExit(ctx, "mirror", 1) // last argument is exit code.
	}

	// extract URLs.
	URLs := ctx.Args()
	srcURL := URLs[0]
	tgtURL := URLs[1]

	/****** Generic rules *******/
	if !ctx.Bool("watch") {
		_, srcContent, err := url2Stat(srcURL)
		// incomplete uploads are not necessary for copy operation, no need to verify for them.
		isIncomplete := false
		if err != nil && !isURLPrefixExists(srcURL, isIncomplete) {
			errorIf(err.Trace(srcURL), "Unable to stat source ‘"+srcURL+"’.")
		}

		if err == nil && !srcContent.Type.IsDir() {
			fatalIf(errInvalidArgument().Trace(srcContent.URL.String(), srcContent.Type.String()), fmt.Sprintf("Source ‘%s’ is not a folder. Only folders are supported by mirror command.", srcURL))
		}
	}

	if len(tgtURL) == 0 && tgtURL == "" {
		fatalIf(errInvalidArgument().Trace(), "Invalid target arguments to mirror command.")
	}

	url := newClientURL(tgtURL)
	if url.Host != "" {
		if !isURLVirtualHostStyle(url.Host) {
			if url.Path == string(url.Separator) {
				fatalIf(errInvalidArgument().Trace(tgtURL),
					fmt.Sprintf("Target ‘%s’ does not contain bucket name.", tgtURL))
			}
		}
	}
	_, _, err := url2Stat(tgtURL)
	// we die on any error other than PathNotFound - destination directory need not exist.
	switch err.ToGoError().(type) {
	case PathNotFound:
	case ObjectMissing:
	default:
		fatalIf(err.Trace(tgtURL), fmt.Sprintf("Unable to stat target ‘%s’.", tgtURL))
	}
}

func deltaSourceTarget(sourceURL string, targetURL string, isForce bool, isFake bool, isRemove bool, isWatch bool, URLsCh chan<- URLs) {
	// source and targets are always directories
	sourceSeparator := string(newClientURL(sourceURL).Separator)
	if !strings.HasSuffix(sourceURL, sourceSeparator) {
		sourceURL = sourceURL + sourceSeparator
	}
	targetSeparator := string(newClientURL(targetURL).Separator)
	if !strings.HasSuffix(targetURL, targetSeparator) {
		targetURL = targetURL + targetSeparator
	}

	// Extract alias and expanded URL
	sourceAlias, sourceURL, _ := mustExpandAlias(sourceURL)
	targetAlias, targetURL, _ := mustExpandAlias(targetURL)

	defer close(URLsCh)

	sourceClnt, err := newClientFromAlias(sourceAlias, sourceURL)
	if err != nil {
		URLsCh <- URLs{Error: err.Trace(sourceAlias, sourceURL)}
		return
	}

	targetClnt, err := newClientFromAlias(targetAlias, targetURL)
	if err != nil {
		URLsCh <- URLs{Error: err.Trace(targetAlias, targetURL)}
		return
	}

	// List both source and target, compare and return values through channel.
	for diffMsg := range objectDifference(sourceClnt, targetClnt, sourceURL, targetURL, isWatch) {
		switch diffMsg.Diff {
		case differInNone:
			// No difference, continue.
			continue
		case differInType:
			URLsCh <- URLs{Error: errInvalidTarget(diffMsg.SecondURL)}
			continue
		case differInSize:
			if !isForce && !isFake {
				// Size differs and force not set
				URLsCh <- URLs{Error: errOverWriteNotAllowed(diffMsg.SecondURL)}
				continue
			}
			sourceSuffix := strings.TrimPrefix(diffMsg.FirstURL, sourceURL)
			// Either available only in source or size differs and force is set
			targetPath := urlJoinPath(targetURL, sourceSuffix)
			sourceContent := diffMsg.firstContent
			targetContent := &clientContent{URL: *newClientURL(targetPath)}
			URLsCh <- URLs{
				SourceAlias:   sourceAlias,
				SourceContent: sourceContent,
				TargetAlias:   targetAlias,
				TargetContent: targetContent,
			}
			continue
		case differInFirst:
			sourceSuffix := strings.TrimPrefix(diffMsg.FirstURL, sourceURL)
			// Either available only in source or size differs and force is set
			targetPath := urlJoinPath(targetURL, sourceSuffix)
			sourceContent := diffMsg.firstContent
			targetContent := &clientContent{URL: *newClientURL(targetPath)}
			URLsCh <- URLs{
				SourceAlias:   sourceAlias,
				SourceContent: sourceContent,
				TargetAlias:   targetAlias,
				TargetContent: targetContent,
			}
		case differInSecond:
			if isRemove {
				// todo(nl5887): I'd all force and fake checks to the the actual mirror / harvest
				if !isForce && !isFake {
					// Object removal not allowed if force is not set.
					URLsCh <- URLs{
						Error: errDeleteNotAllowed(diffMsg.SecondURL),
					}
					continue
				}
				URLsCh <- URLs{
					TargetAlias:   targetAlias,
					TargetContent: diffMsg.secondContent,
				}
			}
			continue
		default:
			URLsCh <- URLs{
				Error: errUnrecognizedDiffType(diffMsg.Diff).Trace(diffMsg.FirstURL, diffMsg.SecondURL),
			}
			continue
		}
	}
}

// Prepares urls that need to be copied or removed based on requested options.
func prepareMirrorURLs(sourceURL string, targetURL string, isForce bool, isFake bool, isWatch bool, isRemove bool) <-chan URLs {
	URLsCh := make(chan URLs)
	go deltaSourceTarget(sourceURL, targetURL, isForce, isFake, isRemove, isWatch, URLsCh)
	return URLsCh
}
