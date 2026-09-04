package brocker

import (
	"ImageCacheProject/internal/util"
	"context"
	"log/slog"
)

type ErrorHandler struct {
	ctx context.Context
	*util.Paths
	errChan <-chan util.Issue
	fmu     *util.FileMutex
}

func (h *ErrorHandler) RunHandler() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case job, ok := <-h.errChan:
			if !ok {
				return
			}
			if job == nil {
				continue
			}
			if job.GetErr() == nil {
				continue
			}
			callback := job.GetFixCallback()
			if callback() != nil {
				err := callback()
				slog.Info("callback failed: ", err.Error())
			}
			slog.Info("error handled: ", job.GetErr().Error())
		}
	}
}

func NewErrorHandler(ctx context.Context, paths *util.Paths, errChan <-chan util.Issue, fmu *util.FileMutex) *ErrorHandler {
	return &ErrorHandler{ctx, paths, errChan, fmu}
}
