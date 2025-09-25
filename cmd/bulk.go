// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	// "github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/packages"
)

const (
	// PackagesDirectory is the name of the package's main manifest file.
	PackagesDirectory = "packages"

	// DataStreamManifestFile is the name of the data stream's manifest file.
	DataStreamManifestFile = "manifest.yml"

	bulkLongDescription = `Use this command to format the package files.

The formatter supports JSON and YAML format, and skips "ingest_pipeline" directories as it's hard to correctly format Handlebars template files. Formatted files are being overwritten.`
)

func setupBulkCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "docs-bulk",
		Short: "Bulk-format things",
		Long:  bulkLongDescription,
		Args:  cobra.NoArgs,
		RunE:  bulkCommandAction,
	}
	cmd.Flags().BoolP(cobraext.FailFastFlagName, "f", false, cobraext.FailFastFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

type Manifest struct {
	Name	string	`yaml:"name"`
	Title	string	`yaml:"title"`
	Version	string	`yaml:"version"`

	Conditions *struct {
		Kibana *struct {
			Version	string `yaml:"version"`
		} `yaml:"kibana"`

	} `yaml:"conditions"`
}

func bulkCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Format the packages (all of them)")

	// find the packages directory
	// loop over each directory under packages/
	// open each manifest and calculate statistics of some things
	packageRoot, found, err := packages.FindPackagesRoot()
	cmd.Printf("found root %v (%v)\n", packageRoot, found)
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}
	if !found {
		return errors.New("package root not found")
	}

	ff, err := cmd.Flags().GetBool(cobraext.FailFastFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FailFastFlagName)
	}

	/*
	err = formatter.Format(packageRoot, ff)
	if err != nil {
		return fmt.Errorf("formatting the integration failed (path: %s, failFast: %t): %w", packageRoot, ff, err)
	}
	*/

	cmd.Printf("fail fast: %v\n", ff)
	cmd.Println("Done")
	return nil
}
