package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/pathutil"
	"github.com/user/cc-box/core/snapshot"
)

func safeJoin(root, relPath string) (string, error) {
	return pathutil.SafeJoin(root, relPath)
}

func safeClaudePath(relPath string) (string, error) {
	if relPath == ".claude.json" {
		return config.ClaudeJSONPath(), nil
	}
	return pathutil.SafeJoin(config.ClaudeDir(), relPath)
}

func claudeExtraFiles() []snapshot.ExtraFile {
	jsonPath := config.ClaudeJSONPath()
	if _, err := os.Stat(jsonPath); err == nil {
		return []snapshot.ExtraFile{{RelPath: ".claude.json", RealPath: jsonPath}}
	}
	return nil
}

func newClaudeScanner(excludePatterns []string) *snapshot.Scanner {
	s := snapshot.NewScanner(config.ClaudeDir(), excludePatterns)
	s.SetExtraFiles(claudeExtraFiles())
	return s
}

func validateSnapshotID(id string) error {
	if id == "" || strings.ContainsRune(id, 0) || strings.Contains(id, "..") || strings.ContainsAny(id, `/\\`) {
		return fmt.Errorf("无效快照 ID: %s", id)
	}
	return nil
}

func snapshotShortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}
