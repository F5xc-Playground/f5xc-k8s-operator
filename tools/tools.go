//go:build tools
// +build tools

// Package tools tracks tool dependencies for go module management.
package tools

import (
	_ "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	_ "sigs.k8s.io/controller-runtime"
	_ "sigs.k8s.io/controller-runtime/tools/setup-envtest"
)
