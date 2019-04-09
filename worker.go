package workerpool

import (
	"context"
	"fmt"
	"time"
)

type WorkerNode struct {
	cancleFunc context.CancelFunc
	worker     Worker
	id         string
	level      int
	signInTime time.Time
	startTime  time.Time
	pos        int
	status     string
}

type Worker interface {
	GetWorkerID() string
	GetWorkerStatus() (WorkerStatus, error)
	Start(ctx context.Context) error
	GetLevel() int
}

const (
	Waiting  = "wait"
	Working  = "ing"
	NotExist = "notexist"
	Over     = "over"
)

type WorkerStatus struct {
	WorkerID   string
	TotalWork  int
	Finished   int
	WorkStatus string
	Pos        int
	RunTime    time.Duration
	WaitTime   time.Duration
}

func (ws WorkerStatus) ToString() string {
	return fmt.Sprintf("%s@%d@%d@%s@%d", ws.WorkerID, ws.TotalWork, ws.Finished, ws.WorkStatus, ws.Pos)
}
