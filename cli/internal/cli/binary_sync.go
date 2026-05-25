package cli

import (
	"fmt"

	"github.com/user/cc-box/core/binary"
	"github.com/user/cc-box/core/snapshot"
	"github.com/user/cc-box/core/webdav"
)

func printSnapshotClaudeRestoreDryRun(client *webdav.Client, key []byte, snap *snapshot.Snapshot) error {
	plan, err := binary.PlanClaudeRestore(client, key, snap, binary.ClaudeRestoreExact)
	if err != nil {
		return err
	}
	switch plan.Action {
	case binary.ClaudeActionDownload:
		fmt.Printf("  ↓ Claude binary %s → %s\n", plan.TargetVersion, plan.TargetPath)
	case binary.ClaudeActionUnavailable:
		fmt.Printf("  ! Claude binary %s 当前平台云端不可用\n", plan.TargetVersion)
	}
	return nil
}

func applySnapshotClaudeRestore(client *webdav.Client, key []byte, snap *snapshot.Snapshot) (bool, error) {
	plan, err := binary.PlanClaudeRestore(client, key, snap, binary.ClaudeRestoreExact)
	if err != nil {
		return false, err
	}
	return applyClaudeRestorePlan(client, key, plan)
}

func applyClaudeRestorePlan(client *webdav.Client, key []byte, plan *binary.ClaudeRestorePlan) (bool, error) {
	switch plan.Action {
	case binary.ClaudeActionSkipAlreadyInstalled, binary.ClaudeActionSkipNoSnapshot, binary.ClaudeActionNeedUserChoice:
		return false, nil
	case binary.ClaudeActionUnavailable:
		return false, fmt.Errorf("快照需要 Claude %s，但云端没有当前平台可用版本", plan.TargetVersion)
	case binary.ClaudeActionDownload:
		fmt.Printf("恢复 Claude binary %s → %s ...\n", plan.TargetVersion, plan.TargetPath)
		err := binary.ApplyClaudeRestore(client, key, plan, func(total, downloaded int64, part, totalParts int) {
			if total <= 0 {
				return
			}
			pct := float64(downloaded) / float64(total) * 100
			fmt.Printf("\r  进度: %.0f%% (%d/%d 分块)", pct, part, totalParts)
		})
		if err != nil {
			return false, err
		}
		if plan.PathConfig != nil && plan.PathConfig.Message != "" {
			if plan.PathConfig.Error != "" {
				fmt.Printf("\n警告：%s\n", plan.PathConfig.Message)
			} else if plan.PathConfig.Enabled {
				fmt.Printf("\nPATH: %s\n", plan.PathConfig.Message)
			}
		}
		fmt.Printf("\n已恢复 Claude binary %s\n", plan.TargetVersion)
		return true, nil
	default:
		return false, fmt.Errorf("未知 Claude binary 恢复动作: %s", plan.Action)
	}
}
