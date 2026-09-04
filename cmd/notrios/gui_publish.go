//go:build gui

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/renesugar/notrios/internal/archivev2"
	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/publish"
	"github.com/renesugar/notrios/internal/store"
)

// Publishing: review exactly what would leave the library, then agree to it.
//
// Publishing is not export. An export preserves enough to restore private
// state; a publication is a sanitized projection — current revisions only, no
// trashed notes, no provenance, no source bundles, stripped revision metadata —
// and it is the one output that rewrites note content, because a link to a
// note you are withholding cannot stay a link. Getting that wrong publishes
// something private, which is why the whole design is built around a review
// somebody actually reads.
//
// Planning goes through this bridge rather than over REST, and not for the
// usual path reason. `notriosctl publish run` re-plans at the moment it runs
// and refuses unless the digest matches the one that was reviewed. For that
// check to mean anything, the plan shown here must be the same plan the run
// recomputes — same profile, same planner, same library. Rebuilding an
// equivalent selection over REST would produce a digest that is equal only by
// luck, and a reviewed-plan check that passes by luck is worse than none.
//
// Profiles are read and never written here. A profile is a curated artefact
// with a target, a selection and a privacy policy; `publish profile save` has
// a dozen flags for it, and a form that quietly got one wrong would produce a
// publication nobody reviewed the shape of. Choosing among profiles somebody
// wrote deliberately is a different act from authoring one.

// PublishProfile is one saved publication, named for a person to choose from.
type PublishProfile struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Target      string `json:"target"`
}

// PublicationPlan is the review: what would be published, what withheld, and
// the digest that binds this review to the run that follows it.
type PublicationPlan struct {
	Profile        string                     `json:"profile"`
	Target         string                     `json:"target"`
	ManifestSHA256 string                     `json:"manifest_sha256"`
	Counts         store.SelectionPlanCounts  `json:"counts"`
	Exclusions     []store.SelectionExclusion `json:"exclusions"`
	Warnings       []string                   `json:"warnings"`
	Truncated      bool                       `json:"truncated"`
	// Command is the exact command line that would do this from a terminal,
	// including the reviewed digest. It is here because a person who wants the
	// terminal should not have to reconstruct a sha256 by hand.
	Command string `json:"command"`
}

// PublicationResult is what a completed publication wrote.
type PublicationResult struct {
	Profile        string `json:"profile"`
	Directory      string `json:"directory"`
	Documents      int    `json:"documents"`
	Objects        int    `json:"objects"`
	ManifestSHA256 string `json:"manifest_sha256"`
}

func (b *NativeUIBridge) publishProfiles() (publish.File, config.Config, error) {
	if b == nil || b.local == nil {
		return publish.File{}, config.Config{}, errNoLocalService
	}
	cfg := b.local.Config
	file, err := publish.Load(publish.DefaultPath(cfg.Data.Directory))
	return file, cfg, err
}

// PublishProfiles lists the publications this library has been taught to make.
func (b *NativeUIBridge) PublishProfiles() ([]PublishProfile, error) {
	file, _, err := b.publishProfiles()
	if err != nil {
		return nil, err
	}
	profiles := make([]PublishProfile, 0, len(file.Profiles))
	for _, profile := range file.Profiles {
		profiles = append(profiles, PublishProfile{
			Name: profile.Name, Description: profile.Description, Target: profile.Target,
		})
	}
	return profiles, nil
}

// PlanPublication reviews one profile and changes nothing.
func (b *NativeUIBridge) PlanPublication(name string) (PublicationPlan, error) {
	file, _, err := b.publishProfiles()
	if err != nil {
		return PublicationPlan{}, err
	}
	profile, err := file.ByName(name)
	if err != nil {
		return PublicationPlan{}, err
	}
	plan, err := b.local.Store.PlanSelection(context.Background(), profile.PlanRequest(1))
	if err != nil {
		return PublicationPlan{}, err
	}
	return PublicationPlan{
		Profile: profile.Name, Target: plan.Target, ManifestSHA256: plan.ManifestSHA256,
		Counts: plan.Counts, Exclusions: plan.Exclusions, Warnings: plan.Warnings,
		Truncated: plan.Truncated,
		Command: fmt.Sprintf("notriosctl publish run --profile %q --reviewed-plan %s <out-dir>",
			profile.Name, plan.ManifestSHA256),
	}, nil
}

// Publish writes the publication, and only the one that was reviewed.
//
// The digest is required rather than optional, and it is checked against a plan
// computed now rather than against the one the interface displayed. A library
// edited between the review and this call produces a different digest and the
// publication is refused — which is the point: what somebody agreed to was a
// description of a library, not a button.
func (b *NativeUIBridge) Publish(name, reviewedDigest, path string) (PublicationResult, error) {
	destination, err := b.directory(path)
	if err != nil {
		return PublicationResult{}, err
	}
	file, _, err := b.publishProfiles()
	if err != nil {
		return PublicationResult{}, err
	}
	profile, err := file.ByName(name)
	if err != nil {
		return PublicationResult{}, err
	}
	// Refuse a folder that already holds something else, and say why in terms
	// of the thing being asked for.
	//
	// archivev2.Export checks this too and refuses correctly, but it answers in
	// its own vocabulary: 'is not an archive-v2 directory (unexpected entry
	// ".~lock...#")'. That is accurate and tells a person nothing about what to
	// do next. Somebody pointed the folder chooser at their Downloads folder and
	// got exactly that. The export's check remains the real guard; this one
	// exists to be understood.
	if err := requireEmptyOrArchive(destination); err != nil {
		return PublicationResult{}, err
	}

	ctx := context.Background()
	plan, err := b.local.Store.PlanSelection(ctx, profile.PlanRequest(1))
	if err != nil {
		return PublicationResult{}, err
	}
	if err := publish.RequireReviewedPlan(reviewedDigest, plan.ManifestSHA256); err != nil {
		return PublicationResult{}, err
	}
	report, err := archivev2.Export(ctx, b.local.Store, destination, archivev2.ExportOptions{
		Target:    profile.Target,
		Selection: profile.Selection,
		Policy:    profile.Policy,
	})
	if err != nil {
		return PublicationResult{}, err
	}
	// The command line makes this same check after exporting. An export whose
	// own manifest disagrees with the reviewed plan has published something
	// other than what was agreed to, and saying so afterwards is the only
	// remaining opportunity to notice.
	if report.SelectionManifestSHA256 != plan.ManifestSHA256 {
		return PublicationResult{}, fmt.Errorf(
			"the publication does not match the reviewed plan (%s written, %s reviewed)",
			report.SelectionManifestSHA256, plan.ManifestSHA256)
	}
	return PublicationResult{
		Profile: profile.Name, Directory: destination,
		Documents: report.SelectedDocuments, Objects: report.Objects,
		ManifestSHA256: report.SelectionManifestSHA256,
	}, nil
}

// requireEmptyOrArchive rejects a destination that holds unrelated files.
//
// A publication is a directory of its own: an empty folder, or one holding a
// previous publication to replace. Writing into a folder of somebody's
// documents would scatter archive objects among them, and the mistake is easy
// to make with a folder chooser open.
func requireEmptyOrArchive(destination string) error {
	entries, err := os.ReadDir(destination)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	for _, entry := range entries {
		if entry.Name() == "manifest.json" {
			return nil
		}
	}
	return fmt.Errorf("%s already contains other files. Publishing writes an archive into a folder of "+
		"its own: choose an empty folder, or one holding a publication to replace", destination)
}
