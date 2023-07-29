// Package bkl implements a layered configuration language parser.
//
//   - Language & tool documentation: https://bkl.gopatchy.io/
//   - Go library source: https://github.com/gopatchy/bkl
//   - Go library documentation: https://pkg.go.dev/github.com/gopatchy/bkl
package bkl

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/gopatchy/bkl/polyfill"
)

// A Parser reads input documents, merges layers, and generates outputs.
//
// # Terminology
//   - Each Parser can read multiple files
//   - Each file represents a single layer
//   - Each file contains one or more documents
//   - Each document generates one or more outputs
//
// # Directive Evaluation Order
//
// Directive evaluation order can matter, e.g. if you $merge a subtree that
// contains an $output directive.
//
// Merge phase 1 (load)
//   - $parent
//
// Merge phase 2 (evaluate)
//   - $env
//
// Merge phase 3 (merge)
//   - $delete
//   - $replace: true
//
// Output phase 1 (process)
//   - $merge
//   - $replace: string
//   - $output: false
//   - $encode
//
// Output phase 2 (output)
//   - $output: true
type Parser struct {
	docs  []any
	debug bool
}

// New creates and returns a new [Parser] with an empty starting document set.
//
// New always succeeds and returns a Parser instance.
func New() *Parser {
	return &Parser{}
}

// SetDebug enables or disables debug log output to stderr.
func (p *Parser) SetDebug(debug bool) {
	p.debug = debug
}

// MergeParser applies other's internal document state to ours using bkl's
// merge semantics.
func (p *Parser) MergeParser(other *Parser) error {
	for i, doc := range other.docs {
		err := p.MergePatch(i, doc)
		if err != nil {
			return err
		}
	}

	return nil
}

// MergePatch applies the supplied patch to the [Parser]'s current internal
// document state at the specified document index using bkl's merge
// semantics.
//
// index is only a hint; if the patch contains a $match entry, that is used
// instead.
func (p *Parser) MergePatch(index int, patch any) error {
	if patchMap, ok := patch.(map[string]any); ok {
		var m any

		m, patch = popMapValue(patchMap, "$match")
		if m != nil {
			found := false

			for i, doc := range p.docs {
				if match(doc, m) {
					found = true

					err := p.MergePatch(i, patch)
					if err != nil {
						return err
					}
				}
			}

			if !found {
				return fmt.Errorf("%#v: %w", m, ErrNoMatchFound)
			}

			return nil
		}
	}

	if index >= len(p.docs) {
		p.docs = append(p.docs, make([]any, index-len(p.docs)+1)...)
	}

	merged, err := merge(p.docs[index], patch)
	if err != nil {
		return err
	}

	p.docs[index] = merged

	return nil
}

// MergeFile parses the file at path and merges its contents into the
// [Parser]'s document state using bkl's merge semantics.
func (p *Parser) MergeFile(path string) error {
	p.log("[%s] loading", path)

	f, err := loadFile(path)
	if err != nil {
		return err
	}

	for i, doc := range f.docs {
		p.log("[%s] merging", f.path)

		err = p.MergePatch(i, doc)
		if err != nil {
			return fmt.Errorf("[%s:doc%d]: %w", f.path, i, err)
		}
	}

	return nil
}

// MergeFileLayers determines relevant layers from the supplied path and merges
// them in order.
func (p *Parser) MergeFileLayers(path string) error {
	files := []*file{}

	for {
		p.log("[%s] loading", path)

		file, err := loadFile(path)
		if err != nil {
			return err
		}

		files = append(files, file)

		parent, err := file.parent()
		if err != nil {
			return err
		}

		if *parent == baseTemplate {
			break
		}

		path = *parent
	}

	polyfill.SlicesReverse(files)

	for _, f := range files {
		for i, doc := range f.docs {
			p.log("[%s] merging", f.path)

			err := p.MergePatch(i, doc)
			if err != nil {
				return fmt.Errorf("[%s:doc%d]: %w", f.path, i, err)
			}
		}
	}

	return nil
}

// NumDocuments returns the number of documents in the [Parser]'s internal
// state.
func (p *Parser) NumDocuments() int {
	return len(p.docs)
}

// Document returns the parsed, merged (but not processed) tree for the
// document at index.
func (p *Parser) Document(index int) (any, error) {
	if index >= p.NumDocuments() {
		return nil, fmt.Errorf("%d: %w", index, ErrInvalidIndex)
	}

	return p.docs[index], nil
}

// Documents returns the parsed, merged (but not processed) trees for all
// documents.
func (p *Parser) Documents() ([]any, error) {
	return p.docs, nil
}

// OutputDocumentsIndex returns the output objects generated by the document
// at the specified index.
func (p *Parser) OutputDocumentsIndex(index int) ([]any, error) {
	obj, err := p.Document(index)
	if err != nil {
		return nil, err
	}

	docs, err := p.Documents()
	if err != nil {
		return nil, err
	}

	obj, err = Process(obj, obj, docs)
	if err != nil {
		return nil, err
	}

	if obj == nil {
		return nil, nil
	}

	obj, outs, err := findOutputs(obj)
	if err != nil {
		return nil, err
	}

	if len(outs) == 0 {
		outs = append(outs, obj)
	}

	err = validate(obj)
	if err != nil {
		return nil, err
	}

	return outs, nil
}

// OutputDocuments returns the output objects generated by all documents.
func (p *Parser) OutputDocuments() ([]any, error) {
	ret := []any{}

	for i := 0; i < p.NumDocuments(); i++ {
		outs, err := p.OutputDocumentsIndex(i)
		if err != nil {
			return nil, err
		}

		ret = append(ret, outs...)
	}

	return ret, nil
}

// OutputIndex returns the outputs generated by the document at the
// specified index, encoded in the specified format.
func (p *Parser) OutputIndex(index int, ext string) ([][]byte, error) {
	outs, err := p.OutputDocumentsIndex(index)
	if err != nil {
		return nil, err
	}

	f, err := GetFormat(ext)
	if err != nil {
		return nil, err
	}

	encs := [][]byte{}

	for j, out := range outs {
		enc, err := f.Marshal(out)
		if err != nil {
			return nil, fmt.Errorf("[doc%d:out%d]: %w", index, j, err)
		}

		encs = append(encs, enc)
	}

	return encs, nil
}

// Outputs returns all outputs from all documents encoded in the specified
// format.
func (p *Parser) Outputs(ext string) ([][]byte, error) {
	outs := [][]byte{}

	for i := 0; i < p.NumDocuments(); i++ {
		out, err := p.OutputIndex(i, ext)
		if err != nil {
			return nil, err
		}

		outs = append(outs, out...)
	}

	return outs, nil
}

// Output returns all documents encoded in the specified format and merged into
// a stream with ---.
func (p *Parser) Output(ext string) ([]byte, error) {
	outs, err := p.OutputDocuments()
	if err != nil {
		return nil, err
	}

	f, err := GetFormat(ext)
	if err != nil {
		return nil, err
	}

	return f.MarshalStream(outs)
}

// OutputToFile encodes all documents in the specified format and writes them
// to the specified output path.
//
// If format is "", it is inferred from path's file extension.
func (p *Parser) OutputToFile(path, format string) error {
	if format == "" {
		format = ext(path)
	}

	fh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return polyfill.ErrorsJoin(fmt.Errorf("%s: %w", path, ErrOutputFile), err)
	}

	defer fh.Close()

	err = p.OutputToWriter(fh, format)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	return nil
}

// OutputToWriter encodes all documents in the specified format and writes them
// to the specified [io.Writer].
//
// If format is "", it defaults to "json-pretty".
func (p *Parser) OutputToWriter(fh io.Writer, format string) error {
	if format == "" {
		format = "json-pretty"
	}

	out, err := p.Output(format)
	if err != nil {
		return err
	}

	_, err = fh.Write(out)
	if err != nil {
		return polyfill.ErrorsJoin(ErrOutputFile, err)
	}

	return nil
}

func (p *Parser) log(format string, v ...any) {
	if !p.debug {
		return
	}

	log.Printf(format, v...)
}
