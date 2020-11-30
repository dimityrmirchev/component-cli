// +build tools

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	_ "github.com/onsi/ginkgo/ginkgo"
	_ "golang.org/x/lint/golint"
	_ "github.com/golang/mock/mockgen"
)
