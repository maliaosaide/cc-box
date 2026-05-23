package cli

import (
	"fmt"
	"strings"

	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/pathutil"
)

func safeJoin(root, relPath string) (string, error) {
	return pathutil.SafeJoin(root, relPath)
}

func safeClaudePath(relPath string) (string, error) {
	return pathutil.SafeJoin(config.ClaudeDir(), relPath)
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
