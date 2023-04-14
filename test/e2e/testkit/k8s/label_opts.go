//go:build e2e

package k8s

import (
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit"
)

type Labels map[string]string

// WithLabel is a functional option for attaching a label value.
func WithLabel(label, value string) testkit.OptFunc {
	return func(opt testkit.Opt) {
		switch x := opt.(type) {
		case Labels:
			x[label] = value
		}
	}
}

// ProcessLabelOptions returns the map of labels attached using WithLabel.
func ProcessLabelOptions(opts ...testkit.OptFunc) Labels {
	labels := make(Labels)

	for _, opt := range opts {
		opt(labels)
	}

	return labels
}