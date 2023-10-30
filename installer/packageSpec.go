package installer

import (
	"fmt"
	"github.com/Masterminds/semver/v3"
	"strings"
)

type PackageSpec struct {
	Owner   string
	Project string
	Version string
}

func ParsePackageSpec(packageSpec string) (*PackageSpec, error) {
	p := strings.LastIndex(packageSpec, "@")
	if p == -1 {
		return nil, fmt.Errorf("version not specified")
	}
	version := packageSpec[p+1:]
	_, err := semver.NewVersion(version)
	if err != nil {
		return nil, err
	}
	ownerAndProject := packageSpec[0:p]
	parts := strings.Split(ownerAndProject, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid owner/project")
	}
	owner := parts[0]
	project := parts[1]

	return &PackageSpec{
		Owner:   owner,
		Project: project,
		Version: version,
	}, nil
}

func (pkgSpec *PackageSpec) String() string {
	return fmt.Sprintf("%s/%s@%s", pkgSpec.Owner, pkgSpec.Project, pkgSpec.Version)
}
