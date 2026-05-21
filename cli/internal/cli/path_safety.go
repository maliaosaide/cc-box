package cli

import (
	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/pathutil"
)

func safeJoin(root, relPath string) (string, error) {
	return pathutil.SafeJoin(root, relPath)
}

func safeClaudePath(relPath string) (string, error) {
	return pathutil.SafeJoin(config.ClaudeDir(), relPath)
}
