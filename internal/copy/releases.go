package copy

import (
	"fmt"
	"io"

	gh "github.com/google/go-github/v58/github"
	ghclient "github.com/jpmicrosoft/vcopy/internal/github"
)

// CopyReleases migrates all releases and their assets from source to target.
func CopyReleases(src, tgt *ghclient.Client, srcOwner, srcRepo, tgtOwner, tgtRepo string, verbose bool) error {
	return syncReleases(src, tgt, srcOwner, srcRepo, tgtOwner, tgtRepo, verbose, false)
}

// SyncReleases copies only releases that don't already exist in the target.
// Existing releases in the target are preserved.
func SyncReleases(src, tgt *ghclient.Client, srcOwner, srcRepo, tgtOwner, tgtRepo string, verbose bool) error {
	return syncReleases(src, tgt, srcOwner, srcRepo, tgtOwner, tgtRepo, verbose, true)
}

// CleanTargetReleases deletes all releases in the target repo that don't exist
// in the source. Used by --force to prevent orphaned releases after mirror push.
func CleanTargetReleases(src, tgt *ghclient.Client, srcOwner, srcRepo, tgtOwner, tgtRepo string, verbose bool) error {
	srcReleases, err := src.ListReleases(srcOwner, srcRepo)
	if err != nil {
		return fmt.Errorf("failed to list source releases: %w", err)
	}
	srcTags := make(map[string]bool)
	for _, r := range srcReleases {
		srcTags[r.GetTagName()] = true
	}

	tgtReleases, err := tgt.ListReleases(tgtOwner, tgtRepo)
	if err != nil {
		return fmt.Errorf("failed to list target releases: %w", err)
	}

	var deleted int
	for _, r := range tgtReleases {
		if !srcTags[r.GetTagName()] {
			if verbose {
				fmt.Printf("  Deleting orphaned release: %s (id %d)\n", r.GetTagName(), r.GetID())
			}
			if err := tgt.DeleteRelease(tgtOwner, tgtRepo, r.GetID()); err != nil {
				fmt.Printf("  Warning: failed to delete release %s: %v\n", r.GetTagName(), err)
				continue
			}
			deleted++
		}
	}
	if deleted > 0 {
		fmt.Printf("  Cleaned up %d orphaned releases from target\n", deleted)
	}
	return nil
}

func syncReleases(src, tgt *ghclient.Client, srcOwner, srcRepo, tgtOwner, tgtRepo string, verbose, incrementalOnly bool) error {
	releases, err := src.ListReleases(srcOwner, srcRepo)
	if err != nil {
		return fmt.Errorf("failed to list source releases: %w", err)
	}

	// When incremental, build a set of existing target releases to skip
	existingTags := make(map[string]bool)
	if incrementalOnly {
		tgtReleases, err := tgt.ListReleases(tgtOwner, tgtRepo)
		if err != nil {
			if verbose {
				fmt.Printf("  Warning: could not list target releases: %v\n", err)
			}
		} else {
			for _, r := range tgtReleases {
				existingTags[r.GetTagName()] = true
			}
			if verbose {
				fmt.Printf("  Found %d existing releases in target, will skip those\n", len(existingTags))
			}
		}
	}

	var copied int
	for _, rel := range releases {
		if incrementalOnly && existingTags[rel.GetTagName()] {
			if verbose {
				fmt.Printf("  Skipping existing release: %s\n", rel.GetTagName())
			}
			continue
		}

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
			if verbose {
				fmt.Printf("  Warning: failed to create release %s: %v\n", rel.GetTagName(), err)
			}
			continue
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

			uploadFile, uploadErr := ghclient.NewUploadFile(resp.Body, asset.GetName(), int64(asset.GetSize()))
			resp.Body.Close()
			if uploadErr != nil {
				if verbose {
					fmt.Printf("    Warning: failed to prepare asset %s for upload: %v\n", asset.GetName(), uploadErr)
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
		copied++
	}

	if incrementalOnly {
		fmt.Printf("  Synced %d new releases (%d already existed)\n", copied, len(existingTags))
	} else {
		fmt.Printf("  Copied %d releases\n", copied)
	}
	return nil
}

// ensure io is used
var _ = io.EOF
