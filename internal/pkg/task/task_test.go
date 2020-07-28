package task

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestEarliestStartTime(t *testing.T) {
	now := time.Now()
	nextDay := now.AddDate(0, 0, 1)
	theDayAfter := now.AddDate(0, 0, 2)

	inTasks := []*Task{
		{
			TaskARN:    "task-1",
			ClusterARN: "cluster",
			StartedAt:  &nextDay,
		},
		{
			TaskARN:    "task-2",
			ClusterARN: "cluster",
			StartedAt:  &theDayAfter,
		},
		{
			TaskARN:    "task-3",
			ClusterARN: "cluster",
			StartedAt:  &now,
		},
	}

	got := EarliestStartTime(inTasks)
	require.Equal(t, now.Unix(), got.Unix())
}
