// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package componentreferences

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	cdv2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	cdvalidation "github.com/gardener/component-spec/bindings-go/apis/v2/validation"
	"github.com/gardener/component-spec/bindings-go/ctf"
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/projectionfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/validation/field"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gardener/component-cli/pkg/commands/constants"
	"github.com/gardener/component-cli/pkg/logger"
)

// Options defines the options that are used to add resources to a component descriptor
type Options struct {
	// ComponentArchivePath is the path to the component descriptor
	ComponentArchivePath string

	// either components can be added by a yaml resource template or by input flags

	// ComponentReferenceObjectPath defines the path to the resources defined as yaml or json
	ComponentReferenceObjectPath string
}

// NewAddCommand creates a command to add additional resources to a component descriptor.
func NewAddCommand(ctx context.Context) *cobra.Command {
	opts := &Options{}
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Adds a component reference to a component descriptor",
		Long: `
add adds component references to the defined component descriptor.
The component references can be defined in a file or given through stdin.

The component references are expected to be a multidoc yaml of the following form

<pre>

---
name: 'ubuntu'
componentName: 'github.com/gardener/ubuntu'
version: 'v0.0.1'
...
---
name: 'myref'
componentName: 'github.com/gardener/other'
version: 'v0.0.2'
...

</pre>
`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := opts.Complete(args); err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}

			if err := opts.Run(ctx, logger.Log, osfs.New()); err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
		},
	}

	opts.AddFlags(cmd.Flags())

	return cmd
}

func (o *Options) Run(ctx context.Context, log logr.Logger, fs vfs.FileSystem) error {
	compDescFilePath := filepath.Join(o.ComponentArchivePath, ctf.ComponentDescriptorFileName)

	// add the input to the ctf format
	archiveFs, err := projectionfs.New(fs, o.ComponentArchivePath)
	if err != nil {
		return fmt.Errorf("unable to create projectionfilesystem: %w", err)
	}
	archive, err := ctf.NewComponentArchiveFromFilesystem(archiveFs)
	if err != nil {
		return fmt.Errorf("unable to parse component archive from %s: %w", compDescFilePath, err)
	}

	refs, err := o.generateComponentReferences(fs, archive.ComponentDescriptor)
	if err != nil {
		return err
	}

	for _, ref := range refs {
		if errList := cdvalidation.ValidateComponentReference(field.NewPath(""), ref); len(errList) != 0 {
			return fmt.Errorf("invalid component reference: %w", errList.ToAggregate())
		}
		id := archive.ComponentDescriptor.GetComponentReferenceIndex(ref)
		if id != -1 {
			archive.ComponentDescriptor.ComponentReferences[id] = ref
		} else {
			archive.ComponentDescriptor.ComponentReferences = append(archive.ComponentDescriptor.ComponentReferences, ref)
		}
		log.V(3).Info(fmt.Sprintf("Successfully added component references %q to component descriptor", ref.Name))
	}

	if err := cdvalidation.Validate(archive.ComponentDescriptor); err != nil {
		return fmt.Errorf("invalid component descriptor: %w", err)
	}

	data, err := yaml.Marshal(archive.ComponentDescriptor)
	if err != nil {
		return fmt.Errorf("unable to encode component descriptor: %w", err)
	}
	if err := vfs.WriteFile(fs, compDescFilePath, data, 06444); err != nil {
		return fmt.Errorf("unable to write modified comonent descriptor: %w", err)
	}
	fmt.Printf("Successfully added component references to component descriptor")
	return nil
}

func (o *Options) Complete(args []string) error {

	// default component path to env var
	if len(o.ComponentArchivePath) == 0 {
		o.ComponentArchivePath = filepath.Dir(os.Getenv(constants.ComponentDescriptorPathEnvName))
	}

	return o.validate()
}

func (o *Options) validate() error {
	if len(o.ComponentArchivePath) == 0 {
		return errors.New("component descriptor path must be provided")
	}
	return nil
}

func (o *Options) AddFlags(set *pflag.FlagSet) {
	set.StringVar(&o.ComponentArchivePath, "comp-desc", "", "path to the component descriptor directory")

	// specify the resource
	set.StringVarP(&o.ComponentReferenceObjectPath, "resource", "r", "", "The path to the resources defined as yaml or json")
}

// generateComponentReferences parses component references from the given path and stdin.
func (o *Options) generateComponentReferences(fs vfs.FileSystem, cd *cdv2.ComponentDescriptor) ([]cdv2.ComponentReference, error) {
	resources := make([]cdv2.ComponentReference, 0)
	if len(o.ComponentReferenceObjectPath) != 0 {
		resourceObjectReader, err := fs.Open(o.ComponentReferenceObjectPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read resource object from %s: %w", o.ComponentReferenceObjectPath, err)
		}
		defer resourceObjectReader.Close()
		resources, err = generateComponentReferenceFromReader(resourceObjectReader)
		if err != nil {
			return nil, fmt.Errorf("unable to read resources from %s: %w", o.ComponentReferenceObjectPath, err)
		}
	}

	stdinResources, err := generateComponentReferenceFromReader(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("unable to read from stdin: %w", err)
	}
	return append(resources, stdinResources...), nil
}

// generateResourcesFromPath generates a resource given resource options and a resource template file.
func generateComponentReferenceFromReader(reader io.Reader) ([]cdv2.ComponentReference, error) {
	refs := make([]cdv2.ComponentReference, 0)
	yamldecoder := yamlutil.NewYAMLOrJSONDecoder(reader, 1024)
	for {
		ref := cdv2.ComponentReference{}
		if err := yamldecoder.Decode(&ref); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("unable to decode ref: %w", err)
		}
		refs = append(refs, ref)
	}

	return refs, nil
}
