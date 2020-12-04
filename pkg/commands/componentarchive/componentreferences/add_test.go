// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package componentreferences_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	cdv2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/codec"
	"github.com/gardener/component-spec/bindings-go/ctf"
	testlog "github.com/go-logr/logr/testing"
	"github.com/mandelsoft/vfs/pkg/layerfs"
	"github.com/mandelsoft/vfs/pkg/memoryfs"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/projectionfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/gardener/component-cli/pkg/commands/componentarchive/componentreferences"
	"github.com/gardener/component-cli/pkg/commands/componentarchive/resources"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ComponentReferences Test Suite")
}

var _ = Describe("Add", func() {

	var testdataFs vfs.FileSystem

	BeforeEach(func() {
		fs, err := projectionfs.New(osfs.New(), "./testdata")
		Expect(err).ToNot(HaveOccurred())
		testdataFs = layerfs.New(memoryfs.New(), fs)
	})

	It("should add a reference defined by a file", func() {

		opts := &componentreferences.Options{
			ComponentArchivePath:         "./00-component",
			ComponentReferenceObjectPath: "./resources/00-ref.yaml",
		}

		Expect(opts.Run(context.TODO(), testlog.NullLogger{}, testdataFs)).To(Succeed())

		data, err := vfs.ReadFile(testdataFs, filepath.Join(opts.ComponentArchivePath, ctf.ComponentDescriptorFileName))
		Expect(err).ToNot(HaveOccurred())

		cd := &cdv2.ComponentDescriptor{}
		Expect(codec.Decode(data, cd)).To(Succeed())

		Expect(cd.ComponentReferences).To(HaveLen(1))
		Expect(cd.ComponentReferences[0]).To(MatchFields(IgnoreExtras, Fields{
			"Name":          Equal("ubuntu"),
			"ComponentName": Equal("github.com/gardener/ubuntu"),
			"Version":       Equal("v0.0.1"),
		}))
	})

	It("should add a resource defined by stdin", func() {
		oldstdin := os.Stdin
		defer func() {
			os.Stdin = oldstdin
		}()
		r, w, err := os.Pipe()
		Expect(err).ToNot(HaveOccurred())
		_, err = w.WriteString(`
name: 'ubuntu'
componentName: 'github.com/gardener/ubuntu'
version: 'v0.0.1'
`)
		Expect(err).ToNot(HaveOccurred())
		Expect(w.Close()).To(Succeed())
		os.Stdin = r

		opts := &componentreferences.Options{
			ComponentArchivePath: "./00-component",
		}

		Expect(opts.Run(context.TODO(), testlog.NullLogger{}, testdataFs)).To(Succeed())

		data, err := vfs.ReadFile(testdataFs, filepath.Join(opts.ComponentArchivePath, ctf.ComponentDescriptorFileName))
		Expect(err).ToNot(HaveOccurred())

		cd := &cdv2.ComponentDescriptor{}
		Expect(codec.Decode(data, cd)).To(Succeed())

		Expect(cd.ComponentReferences).To(HaveLen(1))
		Expect(cd.ComponentReferences[0]).To(MatchFields(IgnoreExtras, Fields{
			"Name":          Equal("ubuntu"),
			"ComponentName": Equal("github.com/gardener/ubuntu"),
			"Version":       Equal("v0.0.1"),
		}))
	})

	It("should add multiple reference defined by a multi doc file", func() {

		opts := &componentreferences.Options{
			ComponentArchivePath:         "./00-component",
			ComponentReferenceObjectPath: "./resources/01-multi-doc.yaml",
		}

		Expect(opts.Run(context.TODO(), testlog.NullLogger{}, testdataFs)).To(Succeed())

		data, err := vfs.ReadFile(testdataFs, filepath.Join(opts.ComponentArchivePath, ctf.ComponentDescriptorFileName))
		Expect(err).ToNot(HaveOccurred())

		cd := &cdv2.ComponentDescriptor{}
		Expect(codec.Decode(data, cd)).To(Succeed())

		Expect(cd.ComponentReferences).To(HaveLen(2))
		Expect(cd.ComponentReferences[0]).To(MatchFields(IgnoreExtras, Fields{
			"Name":          Equal("ubuntu"),
			"ComponentName": Equal("github.com/gardener/ubuntu"),
			"Version":       Equal("v0.0.1"),
		}))
		Expect(cd.ComponentReferences[1]).To(MatchFields(IgnoreExtras, Fields{
			"Name":          Equal("myref"),
			"ComponentName": Equal("github.com/gardener/other"),
			"Version":       Equal("v0.0.2"),
		}))
	})

	It("should throw an error if an invalid resource is defined", func() {
		opts := &resources.Options{
			ComponentArchivePath: "./00-component",
			ResourceObjectPath:   "./resources/10-invalid.yaml",
		}

		Expect(opts.Run(context.TODO(), testlog.NullLogger{}, testdataFs)).To(HaveOccurred())

		data, err := vfs.ReadFile(testdataFs, filepath.Join(opts.ComponentArchivePath, ctf.ComponentDescriptorFileName))
		Expect(err).ToNot(HaveOccurred())
		cd := &cdv2.ComponentDescriptor{}
		Expect(codec.Decode(data, cd)).To(Succeed())
		Expect(cd.ComponentReferences).To(HaveLen(0))
	})

})
