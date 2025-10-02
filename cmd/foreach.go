// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"

	"path/filepath"

	semmver "github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	// "github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/packages"
)

const (
	foreachLongDescription = `Use this command to format the package files.

The formatter supports JSON and YAML format, and skips "ingest_pipeline" directories as it's hard to correctly format Handlebars template files. Formatted files are being overwritten.`
)

func setupForeachCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "foreach",
		Short: "Perform some action for each given package",
		Long:  foreachLongDescription,
		Args:  cobra.NoArgs,
		RunE:  foreachCommandAction,
	}
	cmd.Flags().BoolP(cobraext.FailFastFlagName, "f", false, cobraext.FailFastFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func foreachCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Format the packages (all of them)")

	// find the packages directory
	// loop over each directory under packages/
	// open each manifest and calculate statistics of some things
	packagesRoot, found, err := packages.FindPackagesRoot()
	cmd.Printf("found root %v (%v)\n", packagesRoot, found)
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}
	if !found {
		return errors.New("package root not found")
	}

	// loop over each directory in the packagesRoot
	manifests, err := filepath.Glob(filepath.Join(packagesRoot, "*", packages.PackageManifestFile))
	if err != nil {
		return fmt.Errorf("failed matching files with manifest definitions: %w", err)
	}

	// read the manifest file in the integration/package
	// func ReadPackageManifest(path string) (*PackageManifest, error) {
	ff, err := cmd.Flags().GetBool(cobraext.FailFastFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FailFastFlagName)
	}

	for _, file := range manifests {
		mani, err := packages.ReadPackageManifest(file)
		if err != nil {
			if ff {
				return fmt.Errorf("reading file failed (path: %s): %w", file, err)
			}
			cmd.Printf("failed to read file (path: %s): %w\n", file, err)
		}

		cmd.Printf("manifest for package name %s;;", mani.Title)
		constraint, err := semmver.NewConstraint(mani.Conditions.Kibana.Version)
		if err != nil {
			cmd.Printf("Failed to create constraint from %v: %w\n", mani.Conditions.Kibana.Version, err)
		}

		cmd.Printf("Have a valid constraint: %s\n", (*constraint).String())


		/*
		   type PackageManifest struct {
		   	SpecVersion     string           `config:"format_version" json:"format_version" yaml:"format_version"`
		   	Name            string           `config:"name" json:"name" yaml:"name"`
		   	Title           string           `config:"title" json:"title" yaml:"title"`
		   	Type            string           `config:"type" json:"type" yaml:"type"`
		   	Version         string           `config:"version" json:"version" yaml:"version"`
		   	Source          Source           `config:"source" json:"source" yaml:"source"`
		   	Conditions      Conditions       `config:"conditions" json:"conditions" yaml:"conditions"`
		   	Discovery       Discovery        `config:"discovery" json:"discovery" yaml:"discovery"`
		   	PolicyTemplates []PolicyTemplate `config:"policy_templates" json:"policy_templates" yaml:"policy_templates"`
		   	Vars            []Variable       `config:"vars" json:"vars" yaml:"vars"`
		   	Owner           Owner            `config:"owner" json:"owner" yaml:"owner"`
		   	Description     string           `config:"description" json:"description" yaml:"description"`
		   	License         string           `config:"license" json:"license" yaml:"license"`
		   	Categories      []string         `config:"categories" json:"categories" yaml:"categories"`
		   	Agent           Agent            `config:"agent" json:"agent" yaml:"agent"`
		   	Elasticsearch   *Elasticsearch   `config:"elasticsearch" json:"elasticsearch" yaml:"elasticsearch"`
		   }
		*/
		cmd.Printf("  version: %s\n", mani.Version)
		cmd.Printf("  owner: %s\n", mani.Owner)
		cmd.Printf("  license: %s\n", mani.License)

	}

	/*
		err = formatter.Format(packagesRoot, ff)
		if err != nil {
			return fmt.Errorf("formatting the integration failed (path: %s, failFast: %t): %w", packagesRoot, ff, err)
		}
	*/

	cmd.Printf("fail fast: %v\n", ff)
	cmd.Println("Done")
	return nil
}
