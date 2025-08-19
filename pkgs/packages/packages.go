// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	int_pkgs "github.com/elastic/elastic-package/internal/packages"
)

const (
	// PackageManifestFile is the name of the package's main manifest file.
	PackageManifestFile = int_pkgs.PackageManifestFile

	// DataStreamManifestFile is the name of the data stream's manifest file.
	DataStreamManifestFile = int_pkgs.DataStreamManifestFile
)

// MustFindPackageRoot finds and returns the path to the root folder of a package.
// It fails with an error if the package root can't be found.
func MustFindPackageRoot() (string, error) {
	return int_pkgs.MustFindPackageRoot()
}

// FindPackageRoot finds and returns the path to the root folder of a package from the working directory.
func FindPackageRoot() (string, bool, error) {
	return int_pkgs.FindPackageRoot()
}

// FindPackageRootFrom finds and returns the path to the root folder of a package from a given directory.
func FindPackageRootFrom(fromDir string) (string, bool, error) {
	return int_pkgs.FindPackageRootFrom(fromDir)
}

// FindDataStreamRootForPath finds and returns the path to the root folder of a data stream.
func FindDataStreamRootForPath(workDir string) (string, bool, error) {
	return int_pkgs.FindDataStreamRootForPath(workDir)
}

