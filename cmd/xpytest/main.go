package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	xpytest_proto "github.com/pfnet-research/xpytest/proto"

	"github.com/pfnet-research/xpytest/pkg/pytest"
	"github.com/pfnet-research/xpytest/pkg/reporter"
	"github.com/pfnet-research/xpytest/pkg/xpytest"
)

var python = flag.String("python", "python3", "python command")
var markerExpression = flag.String("m", "not slow", "pytest marker expression")
var retry = flag.Int("retry", 2, "number of retries")
var credential = flag.String(
	"credential", "", "JSON credential file for Google")
var spreadsheetID = flag.String("spreadsheet_id", "", "spreadsheet ID to edit")
var hint = flag.String("hint", "", "hint file")
var bucket = flag.Int("bucket", 1, "number of buckets")
var thread = flag.Int("thread", 0, "number of threads per bucket")

func main() {
	flag.Parse()
	ctx := context.Background()

	base := pytest.NewPytest(*python)
	base.MarkerExpression = *markerExpression
	base.Retry = *retry
	base.Deadline = time.Minute
	xt := xpytest.NewXpytest(base)

	r, err := func() (reporter.Reporter, error) {
		if *spreadsheetID == "" {
			return nil, nil
		}
		if *credential != "" {
			return reporter.NewSheetsReporterWithCredential(
				ctx, *credential, *spreadsheetID)
		}
		return reporter.NewSheetsReporter(ctx, *spreadsheetID)
	}()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize reporter: %s", err))
	}
	if r != nil {
		r.Log(ctx, fmt.Sprintf("Time: %s", time.Now()))
	}

	if *hint != "" {
		if h, err := xpytest.LoadHintFile(*hint); err != nil {
			panic(fmt.Sprintf(
				"failed to read hint information from file: %s: %s",
				*hint, err))
		} else if err := xt.ApplyHint(h); err != nil {
			panic(fmt.Sprintf("failed to apply hint: %s", err))
		}
	}

	for _, arg := range flag.Args() {
		if err := xt.AddTestsWithFilePattern(arg); err != nil {
			panic(fmt.Sprintf("failed to add tests: %s", err))
		}
	}

	if err := xt.Execute(ctx, *bucket, *thread, r); err != nil {
		panic(fmt.Sprintf("failed to execute: %s", err))
	}

	if xt.Status != xpytest_proto.TestResult_SUCCESS {
		os.Exit(1)
	}
}
