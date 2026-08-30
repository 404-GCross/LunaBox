package updateclient

import (
	"strings"
	"testing"

	"lunabox/updater/updateutils"
)

func TestManifestURLForVersion(t *testing.T) {
	got, err := manifestURLForVersion("https://updates.example.com/v1/releases/1.12.2/manifest", "v1.12.1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://updates.example.com/v1/releases/1.12.1/manifest" {
		t.Fatalf("unexpected manifest URL: %s", got)
	}
}

func TestResolvePatchChain(t *testing.T) {
	oldSHA := strings.Repeat("1", 64)
	middleSHA := strings.Repeat("2", 64)
	finalSHA := strings.Repeat("3", 64)
	patchOldToMiddle := updateutils.PatchArtifact{
		SourceVersion: "1.12.0",
		SourceSHA256:  oldSHA,
		Artifact:      updateutils.Artifact{URL: "https://updates.example.com/old-to-middle.zsdiff", Size: 1, SHA256: strings.Repeat("4", 64), Compression: updateutils.ArtifactCompressionZstd},
	}
	patchMiddleToFinal := updateutils.PatchArtifact{
		SourceVersion: "1.12.1",
		SourceSHA256:  middleSHA,
		Artifact:      updateutils.Artifact{URL: "https://updates.example.com/middle-to-final.zsdiff", Size: 1, SHA256: strings.Repeat("5", 64), Compression: updateutils.ArtifactCompressionZstd},
	}
	target := updateutils.ReleaseFile{Path: "LunaBox.exe", TargetSHA256: finalSHA, TargetSize: 3, Full: updateutils.Artifact{Size: 10}, Patch: &patchMiddleToFinal}
	resolver := func(version string) (*updateutils.ReleaseManifest, updateutils.ReleaseChannel, error) {
		if version != "1.12.1" {
			t.Fatalf("unexpected resolver version: %s", version)
		}
		return &updateutils.ReleaseManifest{Version: version}, updateutils.ReleaseChannel{Files: []updateutils.ReleaseFile{{
			Path: "LunaBox.exe", TargetSHA256: middleSHA, TargetSize: 2, Patch: &patchOldToMiddle,
		}}}, nil
	}

	chain, ok, err := resolvePatchChain(target, "1.12.0", oldSHA, resolver)
	if err != nil || !ok {
		t.Fatalf("expected patch chain, ok=%v err=%v", ok, err)
	}
	if len(chain) != 2 || chain[0].patch.SourceVersion != "1.12.0" || chain[1].patch.SourceVersion != "1.12.1" {
		t.Fatalf("unexpected patch chain: %+v", chain)
	}
}
