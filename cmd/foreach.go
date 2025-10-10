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
	packagesDescription = `--package <package 1> -p <package 2> ...

This will iterate over each listed package to perform the requested action.`

	foreachLongDescription = `Use this command to format the package files.

The formatter supports JSON and YAML format, and skips "ingest_pipeline" directories as it's hard to correctly format Handlebars template files. Formatted files are being overwritten.`

	foreachTestLongDescription = `Use this command to format the package files.

The formatter supports JSON and YAML format, and skips "ingest_pipeline" directories as it's hard to correctly format Handlebars template files. Formatted files are being overwritten.`
)

func setupForeachCommand() *cobraext.Command {
	// need to add discovery flags
	// --modified
	// --packages
	// --auto
	// --constraint [constraint]
	// what should we support?
	// test


	cmd := &cobra.Command{
		Use:   "foreach",
		Short: "Perform some action for each given package",
		Long:  foreachLongDescription,
		RunE:  foreachCommandAction,
	}

	cmd.PersistentFlags().StringSliceP("package", "P", nil, packagesDescription)
	cmd.PersistentFlags().BoolP(cobraext.FailFastFlagName, "f", false, cobraext.FailFastFlagDescription)
	cmd.PersistentFlags().StringP("constraint", "c", "kibana.version==9.1", "the version to verify")

	ecs := &cobra.Command{
		Use:   "updateEcsVersion",
		Short: "Perform some action for each given package",
		Long:  foreachLongDescription,
		RunE:  updateEcsVersionAction,
	}

	cmd.AddCommand(ecs)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func filterByGlob(){}

type Set struct {
	elements map[string]struct{}
}

func NewSet() *Set {
	return &Set{
		elements: make(map[string]struct{}),
	}
}

func (s *Set) Add(value string) {
	s.elements[value] = struct{}{}
}

func (s *Set) Remove(value string) {
	delete(s.elements, value)
}

func (s *Set) Contains(value string) bool {
	_, found := s.elements[value]
	return found
}

func (s *Set) Size() int {
	return len(s.elements)
}

func (s *Set) List() []string {
	keys := make([]string, 0, len(s.elements))
	for key := range s.elements {
		keys = append(keys, key)
	}
	return keys
}

func updateEcsVersionAction(cmd *cobra.Command, args []string) error {
	cmd.Println("go through each package")
	packageSet := NewSet()

	packageGlobs, err := cmd.Flags().GetStringSlice("package")
	cmd.Printf("does any match %v :: %v\n", packageGlobs, err)

	// get a list of all the packages
	// filter each package through all the given globs
	// the matches go into another array

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
	for _, p := range packageGlobs {
		manifests, err := filepath.Glob(filepath.Join(packagesRoot, p, packages.PackageManifestFile))
		if err != nil {
			return fmt.Errorf("failed matching files with manifest definitions: %w", err)
		}

		for _, m := range manifests {
			packageSet.Add(m)
		}

	}

	cmd.Printf("found the following manifests:\n")
	for _, m := range packageSet.List() {
		cmd.Printf("  %s\n", m)
	}
	return nil

}

func foreachCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Format the packages (all of them)")


	if args[0] == "test" {
		cmd.Println(args)
		test := setupTestCommand()
		for _,c := range test.Command.Commands() {
			cmd.Println(c)
			if args[1] == c.Use {
				c.Run(cmd, nil)
			}
		}

	}

	// get our packages
	// get our constraints

	return nil

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
