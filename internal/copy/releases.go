package copy

import (
	"fmt"
	"io"

	gh "github.com/google/go-github/v58/github"
	ghclient "github.com/jpmicrosoft/vcopy/internal/github"
)

// CopyReleases migrates releases and their assets from source to target.
func CopyReleases(src, tgt *ghclient.Client, srcOwner, srcRepo, tgtOwner, tgtRepo string, verbose bool) error {
	releases, err := src.ListReleases(srcOwner, srcRepo)
	if err != nil {
		return fmt.Errorf("failed to list source releases: %w", err)
	}

	for _, rel := range releases {
		if verbose {
			fmt.Printf("  Copying release: %s\n", rel.GetTagName())
		}

		newRelease := &gh.RepositoryRelease{
			TagName:         rel.TagName,
			TargetCommitish: rel.TargetCommitish,
			Name:            rel.Name,
			Body:            rel.Body,
			Draft:           rel.Draft,
			Prerelease:      rel.Prerelease,
		}

		created, err := tgt.CreateRelease(tgtOwner, tgtRepo, newRelease)
		if err != nil {
			return fmt.Errorf("failed to create release %s: %w", rel.GetTagName(), err)
		}

		// Copy release assets
		assets, err := src.ListReleaseAssets(srcOwner, srcRepo, rel.GetID())
		if err != nil {
			if verbose {
				fmt.Printf("  Warning: failed to list assets for release %s: %v\n", rel.GetTagName(), err)
			}
			continue
		}

		for _, asset := range assets {
			if verbose {
				fmt.Printf("    Uploading asset: %s\n", asset.GetName())
			}

			resp, err := src.DownloadReleaseAsset(srcOwner, srcRepo, asset.GetID())
			if err != nil {
				if verbose {
					fmt.Printf("    Warning: failed to download asset %s: %v\n", asset.GetName(), err)
				}
				continue
			}

			uploadFile := ghclient.NewUploadFile(resp.Body, asset.GetName(), int64(asset.GetSize()))
			resp.Body.Close()
			if uploadFile == nil {
				if verbose {
					fmt.Printf("    Warning: failed to prepare asset %s for upload\n", asset.GetName())
				}
				continue
			}
			if err := tgt.UploadReleaseAsset(tgtOwner, tgtRepo, created.GetID(), asset.GetName(), uploadFile); err != nil {
				uploadFile.Cleanup()
				if verbose {
					fmt.Printf("    Warning: failed to upload asset %s: %v\n", asset.GetName(), err)
				}
				continue
			}
			uploadFile.Cleanup()
		}
	}

	fmt.Printf("  Copied %d releases\n", len(releases))
	return nil
}

// ensure io is used
var _ = io.EOF
