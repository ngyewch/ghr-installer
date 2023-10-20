package installer

import (
	"context"
	"fmt"
	"github.com/google/go-github/v56/github"
	"runtime"
)

func Install(packageSpec string) error {
	pkgSpec, err := ParsePackageSpec(packageSpec)
	if err != nil {
		return err
	}

	osClassifier := runtime.GOOS     // TODO always assume runtime.GOOS?
	archClassifier := runtime.GOARCH // TODO always assume runtime.GOARCH?
	// TODO always assume this format?
	// TODO always assume .tar.gz extension?
	downloadFilename := fmt.Sprintf("%s_%s_%s_%s.tar.gz",
		pkgSpec.Project, // TODO always assume filename == projectName?
		pkgSpec.Version, // TODO always assume no prefix?
		osClassifier,
		archClassifier,
	)
	downloadUrl := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/%s",
		pkgSpec.Owner,
		pkgSpec.Project,
		pkgSpec.Version, // TODO always assume 'v' prefix?
		downloadFilename,
	)

	fmt.Println(downloadUrl)

	githubClient := github.NewClient(nil)

	release, _, err := githubClient.Repositories.GetReleaseByTag(context.Background(), pkgSpec.Owner, pkgSpec.Project, fmt.Sprintf("v%s", pkgSpec.Version))
	if err != nil {
		return err
	}
	for _, asset := range release.Assets {
		fmt.Printf("%s -> %s\n", *asset.Name, *asset.URL)
	}

	return nil
}
