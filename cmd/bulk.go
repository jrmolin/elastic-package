// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"

	"encoding/json"

	"path/filepath"

	semmver "github.com/Masterminds/semver/v3"
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

	kibanaVersionSupportLongDescription = `Use this command to list all packages that support the given kibana version (as an argument).

The formatter supports JSON and YAML format, and skips "ingest_pipeline" directories as it's hard to correctly format Handlebars template files. Formatted files are being overwritten.`

)

type PackagesKibana struct {
	Version string `json:"version"`
	Supports []PackageKibana `json:"supports"`
	NotSupports []PackageKibana `json:"doesNotSupport"`
}
type PackageKibana struct {
	Name string `json:"name"`
	Constraints string `json:"constraints"`
}

func setupBulkCommand() *cobraext.Command {
	kibanaList := &cobra.Command{
		Use:   "packagesForKibana",
		Short: "List packages supporting the given kibana version",
		Long:  kibanaVersionSupportLongDescription,
		RunE:  listKibanaPackagesAction,
	}
	kibanaList.Flags().StringP("version", "V", "9.1", "the version to verify")

	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Bulk-format things",
		Long:  bulkLongDescription,
		RunE:  bulkCommandAction,
	}
	cmd.PersistentFlags().BoolP(cobraext.FailFastFlagName, "f", false, cobraext.FailFastFlagDescription)

	cmd.AddCommand(kibanaList)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func listKibanaPackagesAction(cmd *cobra.Command, args []string) error {

	var packageList PackagesKibana
	versionString, err := cmd.Flags().GetString("version")

	if err != nil {
		cmd.Printf("You provided an invalid version (%v): %w\n",
			versionString, err)
		return err
	}

	packageList.Version = versionString

	version, err := semmver.NewVersion(versionString)

	if err != nil {
		cmd.Printf("You provided an invalid version (%v): %w\n",
			versionString, err)
		return err
	}

	// find the packages directory
	// loop over each directory under packages/
	// open each manifest and calculate statistics of some things
	packagesRoot, found, err := packages.FindPackagesRoot()
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

		constraint, err := semmver.NewConstraint(mani.Conditions.Kibana.Version)
		if err != nil {
			cmd.Printf("Failed to create constraint from %v: %w", mani.Conditions.Kibana.Version, err)
		}

		thisPackage := PackageKibana{
			Name: mani.Title,
			Constraints: (*constraint).String(),
		}

		valid, errs := constraint.Validate(version)
		if len(errs) != 0 {
			packageList.NotSupports = append(packageList.NotSupports, thisPackage)
			continue
		}
		if !valid {
			packageList.NotSupports = append(packageList.NotSupports, thisPackage)
			continue
		} else {
			packageList.Supports = append(packageList.Supports, thisPackage)
		}
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
	}

	/*
		err = formatter.Format(packagesRoot, ff)
		if err != nil {
			return fmt.Errorf("formatting the integration failed (path: %s, failFast: %t): %w", packagesRoot, ff, err)
		}
	*/

	jsonData, err := json.MarshalIndent(packageList, "", "  ")
	if err != nil {
		cmd.Printf("failed to marshal to json: %v\n", err)
	} else {
		cmd.Printf("%s\n", string(jsonData))
	}
	return nil
}
func bulkCommandAction(cmd *cobra.Command, args []string) error {
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
		cmd.Printf("kibana conditions: %s\n", mani.Conditions.Kibana.Version)

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
