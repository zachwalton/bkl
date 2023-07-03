package main

import (
	"fmt"
	"os"

	"github.com/gopatchy/bkl"
	"github.com/jessevdk/go-flags"
)

type options struct {
	OutputPath   *string `short:"o" long:"output" description:"output file path"`
	OutputFormat *string `short:"f" long:"format" description:"output format"`
	Verbose      bool    `short:"v" long:"verbose" description:"enable verbose logging"`

	Positional struct {
		InputPaths []string `positional-arg-name:"inputPath" required:"1" description:"input file path"`
	} `positional-args:"yes"`
}

func main() {
	opts := &options{}

	fp := flags.NewParser(opts, flags.Default)

	_, err := fp.Parse()
	if flags.WroteHelp(err) {
		os.Exit(1)
	}

	p := bkl.New()

	if opts.Verbose {
		p.SetDebug(true)
	}

	for _, path := range opts.Positional.InputPaths {
		err := p.MergeFileLayers(path)
		if err != nil {
			fatal("%s", err)
		}
	}

	format := "json"

	if opts.OutputPath != nil {
		format = bkl.Ext(*opts.OutputPath)
	}

	if opts.OutputFormat != nil {
		format = *opts.OutputFormat
	}

	out, err := p.GetOutput(format)
	if err != nil {
		fatal("%s", err)
	}

	fh := os.Stdout

	if opts.OutputPath != nil {
		fh, err = os.OpenFile(*opts.OutputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			fatal("%s: %s", *opts.OutputPath, err)
		}

		defer fh.Close()
	}

	_, err = fh.Write(out)
	if err != nil {
		fatal("output: %s", err)
	}
}

func fatal(format string, v ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", v...)
	os.Exit(1)
}