// 异步操作管理
// goroutine + EventsEmit 进度推送 + 取消支持
package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	opCounter  atomic.Int64
	opCancels  = make(map[int64]context.CancelFunc)
	opCancelMu sync.Mutex
	opResults  = make(map[int64]error)
)

// ProgressEvent 进度事件
type ProgressEvent struct {
	OpID      int64   `json:"opId"`
	Operation string  `json:"operation"`
	Current   int64   `json:"current"`
	Total     int64   `json:"total"`
	Percent   float64 `json:"percent"`
	Part      int     `json:"part"`
	TotalPart int     `json:"totalPart"`
	Message   string  `json:"message"`
}

// OpResult 操作结果事件
type OpResult struct {
	OpID   int64  `json:"opId"`
	Error  string `json:"error,omitempty"`
	Status string `json:"status"` // "success" | "error"
}

// StartAsync 启动异步操作，返回 opId
func (a *App) StartAsync(operation string, fn func(ctx context.Context, opID int64) error) int64 {
	opID := opCounter.Add(1)
	ctx, cancel := context.WithCancel(context.Background())

	opCancelMu.Lock()
	opCancels[opID] = cancel
	opCancelMu.Unlock()

	go func() {
		var err error
		defer func() {
			opCancelMu.Lock()
			delete(opCancels, opID)
			opResults[opID] = err
			opCancelMu.Unlock()
		}()

		err = fn(ctx, opID)

		result := OpResult{OpID: opID, Status: "success"}
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		runtime.EventsEmit(a.ctx, "op:complete", result)
	}()

	return opID
}

// CancelOperation 取消异步操作
func (a *App) CancelOperation(opID int64) {
	opCancelMu.Lock()
	cancel, ok := opCancels[opID]
	opCancelMu.Unlock()

	if ok {
		cancel()
		runtime.EventsEmit(a.ctx, "op:cancelled", OpResult{OpID: opID, Status: "error", Error: "已取消"})
	}
}

// emitProgress 推送进度事件
func (a *App) emitProgress(opID int64, operation string, current, total int64, part, totalPart int, msg string) {
	var pct float64
	if total > 0 {
		pct = float64(current) / float64(total) * 100
	}

	runtime.EventsEmit(a.ctx, "op:progress", ProgressEvent{
		OpID:      opID,
		Operation: operation,
		Current:   current,
		Total:     total,
		Percent:   pct,
		Part:      part,
		TotalPart: totalPart,
		Message:   msg,
	})
}

// ProgressCallback 返回适配 internal 包进度签名的回调
func (a *App) progressCallback(opID int64, operation string) func(total, uploaded int64, part, totalParts int) {
	return func(total, uploaded int64, part, totalParts int) {
		msg := fmt.Sprintf("%d/%d 分块", part, totalParts)
		a.emitProgress(opID, operation, uploaded, total, part, totalParts, msg)
	}
}
