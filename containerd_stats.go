package dexec

import (
	"context"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"time"
)

func getContainerdStats(c *containerd.Client) (Stats, error) {
	ctx := namespaces.WithNamespace(context.Background(), c.DefaultNamespace())

	filters := `labels."wk/owner"==chains`
	containers, err := c.Containers(ctx, filters)
	if err != nil {
		return Stats{}, fmt.Errorf("error getting stats: %w", err)
	}

	stats := Stats{}

	for _, container := range containers {
		if labels, err := container.Labels(ctx); err == nil {
			if deadline, ok := labels["chains/deadline"]; ok {
				if deadlineTime, err := time.Parse(deadline, time.RFC3339); err == nil && time.Now().After(deadlineTime) {
					stats.DeadlineExceeded += 1
				} else if err != nil {
					stats.Errors += 1
				}
			}
		} else {
			stats.Errors += 1
		}
		if task, err := container.Task(ctx, nil); err == nil {
			if status, err := task.Status(ctx); err == nil {
				switch status.Status {
				case containerd.Stopped:
					stats.Stopped += 1
				case containerd.Running:
					stats.Running += 1
				case containerd.Created:
					stats.Created += 1
				case containerd.Paused:
					stats.Paused += 1
				case containerd.Pausing:
					stats.Pausing += 1
				case containerd.Unknown:
					stats.Unknown += 1
				}
			} else {
				stats.Errors += 1
			}
		} else {
			stats.Errors += 1
		}
	}
	return stats, nil
}
