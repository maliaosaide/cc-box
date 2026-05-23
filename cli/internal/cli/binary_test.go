package cli

import (
	"testing"

	"github.com/user/cc-box/core/binary"
)

func TestCollectPruneTargetsOnlyCurrentPlatform(t *testing.T) {
	idx := &binary.Index{Platforms: map[string]binary.PlatformBins{
		"windows-amd64": {
			Claude: &binary.BinaryInfo{
				Current: "1.0.0",
				Versions: map[string]binary.Version{
					"1.0.0": {Hash: "sha256:current"},
					"0.9.0": {Hash: "sha256:old"},
					"0.8.0": {Hash: "sha256:referenced", Refs: 1},
				},
			},
		},
		"darwin-arm64": {
			Claude: &binary.BinaryInfo{
				Current: "1.0.0",
				Versions: map[string]binary.Version{
					"0.9.0": {Hash: "sha256:other-platform"},
				},
			},
		},
	}}

	targets := collectPruneTargets(idx, "windows-amd64")
	if len(targets) != 1 {
		t.Fatalf("targets = %+v, want one current-platform target", targets)
	}
	if targets[0].platform != "windows-amd64" || targets[0].version != "0.9.0" || targets[0].hash != "sha256:old" {
		t.Fatalf("target = %+v", targets[0])
	}
}
