// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier
package processors

import (
	"context"
	"fmt"
	"io"

	cdv2 "github.com/gardener/component-spec/bindings-go/apis/v2"

	"github.com/gardener/component-cli/pkg/transport/process"
)

type labellingProcessor struct {
	labels cdv2.Labels
}

// NewLabellingProcessor returns a processor that appends one or more labels to a resource
func NewLabellingProcessor(labels ...cdv2.Label) process.ResourceStreamProcessor {
	obj := labellingProcessor{
		labels: labels,
	}
	return &obj
}

func (p *labellingProcessor) Process(ctx context.Context, r io.Reader, w io.Writer) error {
	cd, res, resBlobReader, err := process.ReadProcessorMessage(r)
	if err != nil {
		return fmt.Errorf("unable to read processor message: %w", err)
	}
	if resBlobReader != nil {
		defer resBlobReader.Close()
	}

	res.Labels = append(res.Labels, p.labels...)

	if err := process.WriteProcessorMessage(*cd, res, resBlobReader, w); err != nil {
		return fmt.Errorf("unable to write processor message: %w", err)
	}

	return nil
}
