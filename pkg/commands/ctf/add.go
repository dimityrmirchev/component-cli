// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package ctf

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/gardener/component-spec/bindings-go/ctf"
	"github.com/go-logr/logr"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gardener/component-cli/pkg/logger"
)

type AddOptions struct {
	// CTFPath is the path to the directory containing the ctf archive.
	CTFPath string

	ComponentArchives []string
}

// NewAddCommand creates a new definition command to push definitions
func NewAddCommand(ctx context.Context) *cobra.Command {
	opts := &AddOptions{}
	cmd := &cobra.Command{
		Use:   "add [ctf-path] [-f component-archive]...",
		Args:  cobra.RangeArgs(1, 4),
		Short: "Adds component archives to a ctf",
		Run: func(cmd *cobra.Command, args []string) {
			if err := opts.Complete(args); err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}

			if err := opts.Run(ctx, logger.Log, osfs.New()); err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}

			fmt.Print("Successfully uploaded ctf\n")
		},
	}

	opts.AddFlags(cmd.Flags())

	return cmd
}

func (o *AddOptions) Run(ctx context.Context, log logr.Logger, fs vfs.FileSystem) error {
	info, err := fs.Stat(o.CTFPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("unable to get info for %s: %w", o.CTFPath, err)
		}
		log.Info("CTF Archive does not exist creating a new one")

		file, err := fs.OpenFile(o.CTFPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
		if err != nil {
			return fmt.Errorf("unable to open file for %s: %w", o.CTFPath, err)
		}
		tw := tar.NewWriter(file)
		if err := tw.Close(); err != nil {
			return fmt.Errorf("unable to close tarwriter for emtpy tar: %w", err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("unable to close tarwriter for emtpy tar: %w", err)
		}
		info, err = fs.Stat(o.CTFPath)
		if err != nil {
			return fmt.Errorf("unable to get info for %s: %w", o.CTFPath, err)
		}
	}
	if info.IsDir() {
		return fmt.Errorf(`%q is a directory. 
It is expected that the given path points to a CTF Archive`, o.CTFPath)
	}

	ctfArchive, err := ctf.NewCTF(fs, o.CTFPath)
	if err != nil {
		return fmt.Errorf("unable to open ctf at %q: %s", o.CTFPath, err.Error())
	}

	for _, caPath := range o.ComponentArchives {
		file, err := fs.Open(caPath)
		if err != nil {
			return fmt.Errorf("unable to read component archive from %q: %s", caPath, err.Error())
		}
		ca, err := ctf.NewComponentArchiveFromTarReader(file)
		if err != nil {
			return fmt.Errorf("unable to parse component archive from %q: %s", caPath, err.Error())
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("unable to close component archive from %q: %s", caPath, err.Error())
		}
		if err := ctfArchive.AddComponentArchive(ca); err != nil {
			return fmt.Errorf("unable to add component archive %q to ctf: %s", ca.ComponentDescriptor.GetName(), err.Error())
		}
	}
	if err := ctfArchive.Write(); err != nil {
		return fmt.Errorf("unable to write modified ctf archive: %s", err.Error())
	}
	return ctfArchive.Close()
}

func (o *AddOptions) Complete(args []string) error {
	o.CTFPath = args[0]

	if err := o.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate validates push options
func (o *AddOptions) Validate() error {
	if len(o.CTFPath) == 0 {
		return errors.New("a path to the component descriptor must be defined")
	}

	if len(o.ComponentArchives) == 0 {
		return errors.New("no archives to add")
	}

	// todo: validate references exist
	return nil
}

func (o *AddOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringArrayVarP(&o.ComponentArchives, "component-archive", "f", []string{}, "path to the component archives to be added. Note that the component archives have to be tar archives.")
}
