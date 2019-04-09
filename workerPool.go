package workerpool

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

type cacheList struct {
	list       []*WorkerNode
	cur        int
	l          *sync.Mutex
	needResort bool
}

func newCacheList(isNeedResort bool) *cacheList {
	l := make([]*WorkerNode, 10)
	return &cacheList{
		list:       l[:0],
		l:          new(sync.Mutex),
		cur:        0,
		needResort: isNeedResort,
	}
}

func (cl *cacheList) pop() *WorkerNode {
	cl.l.Lock()
	defer cl.l.Unlock()
	if cl.needResort {
		unread := cl.list[cl.cur:]
		sort.Slice(unread, func(i, j int) bool {
			wt1 := time.Now().Sub(unread[i].signInTime).Nanoseconds()
			wt2 := time.Now().Sub(unread[j].signInTime).Nanoseconds()
			if math.Pow(float64(wt1), float64(unread[i].level)) > math.Pow(float64(wt2), float64(unread[j].level)) {
				unread[i].pos, unread[j].pos = unread[j].pos, unread[i].pos
				return true
			}
			return false
		})
	}
	idx := 0
	if len(cl.list)-cl.cur > 0 {
		idx = cl.cur
		cl.cur++
		return cl.list[idx]
	}
	cl.cur = 0
	cl.list = cl.list[:0]
	return nil
}

func (cl *cacheList) push(w *WorkerNode) error {
	cl.l.Lock()
	defer cl.l.Unlock()
	clCap := cap(cl.list)
	clLen := len(cl.list) - cl.cur
	w.pos = clLen + 1
	if l := len(cl.list); l+1 <= clCap {
		cl.list = cl.list[:l+1]
		cl.list[l] = w
		return nil
	}

	if clLen == 0 {
		cl.cur = 0
		cl.list = cl.list[:0]
	}

	if clLen+1 <= clCap/2 {
		copy(cl.list, cl.list[cl.cur:])
		cl.list = cl.list[:clLen+1]
		cl.list[clLen] = w
		cl.cur = 0
		return nil
	} else if clCap*2+1 < int(^uint(0)>>1) {
		buf := make([]*WorkerNode, 2*clCap+1)
		copy(buf, cl.list[cl.cur:])
		cl.list = buf[:clLen+1]
		cl.cur = 0
		cl.list[clLen] = w
		return nil
	} else {
		return errors.New("Cache too large")
	}
}

type LWorkerPool struct {
	indexLock     *sync.RWMutex
	maxConcurrent int
	waitPool      *cacheList
	workerIndex   map[string]*WorkerNode //save all worker in one map and distinguish with status
	cancleCtx     context.Context
	cancleFunc    context.CancelFunc
	workQuatoChan chan struct{}
	cancleChan    chan struct{}
}

//NewLWorkerPool : isNeedResort means cache will resort wait cache with level and wait time, it will be
// inefficiency if there is too many worker is waiting.
func NewLWorkerPool(maxConcurrent int, isNeedResort bool) *LWorkerPool {
	qc := make(chan struct{}, maxConcurrent)
	cc, cf := context.WithCancel(context.Background())
	for i := 0; i < maxConcurrent; i++ {
		qc <- struct{}{}
	}
	wp := &LWorkerPool{
		waitPool:      newCacheList(isNeedResort),
		workerIndex:   make(map[string]*WorkerNode),
		cancleCtx:     cc,
		cancleFunc:    cf,
		workQuatoChan: qc,
		indexLock:     new(sync.RWMutex),
		cancleChan:    make(chan struct{}, 1),
	}
	go wp.loop()
	return wp
}

func (p *LWorkerPool) GetCacheLenCapMem() (int, int) {
	return len(p.waitPool.list), cap(p.waitPool.list)
}

func (p *LWorkerPool) GetWorkStatus(id string) WorkerStatus {
	if w, ok := p.workerIndex[id]; ok {
		if w.status == Working {
			ws, err := w.worker.GetWorkerStatus()
			if err != nil {
				return WorkerStatus{
					WorkerID:   w.id,
					WorkStatus: Working,
					TotalWork:  1,
					Finished:   0,
					WaitTime:   0,
					RunTime:    0,
				}
			}
			ws.WorkStatus = Working
			ws.WaitTime = w.startTime.Sub(w.signInTime)
			ws.RunTime = time.Now().Sub(w.startTime)
			return ws
		} else if w.status == Waiting {
			return WorkerStatus{
				WorkerID:   w.id,
				WorkStatus: Waiting,
				Pos:        w.pos,
				WaitTime:   time.Now().Sub(w.signInTime),
				RunTime:    0,
			}
		}
	}
	return WorkerStatus{
		WorkerID:   id,
		WorkStatus: NotExist,
	}
}

func (p *LWorkerPool) IsWorkExist(id string) WorkerStatus {
	p.indexLock.RLock()
	defer p.indexLock.RUnlock()
	return p.GetWorkStatus(id)
}

func (p *LWorkerPool) GetAllWork() (int, []WorkerStatus) {
	p.indexLock.RLock()
	defer p.indexLock.RUnlock()
	ret := make([]WorkerStatus, 0, len(p.workerIndex))
	for k := range p.workerIndex {
		ret = append(ret, p.GetWorkStatus(k))
	}
	return len(ret), ret
}

func (p *LWorkerPool) SignInWork(w Worker) (*WorkerStatus, error) {
	p.indexLock.Lock()
	defer p.indexLock.Unlock()
	wn := &WorkerNode{
		worker:     w,
		level:      w.GetLevel(),
		signInTime: time.Now(),
		id:         w.GetWorkerID(),
		status:     Waiting}
	status := p.GetWorkStatus(w.GetWorkerID())
	if status.WorkStatus != NotExist {
		return &status, fmt.Errorf("Work is exist")
	}
	p.workerIndex[w.GetWorkerID()] = wn
	// p.waitPoolIndex[]
	err := p.waitPool.push(wn)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (p *LWorkerPool) loop() {
	for {
		select {
		case <-p.cancleChan:
			p.cancleFunc()
		case <-p.workQuatoChan:
			//pop one work from pool with leve and other order
			w := p.waitPool.pop()
			if w == nil {
				time.Sleep(time.Second)
				p.workQuatoChan <- struct{}{}
				continue
			}
			go p.doWork(w)
			// p.ingLock.Lock()
			// // p.workingPool = append(p.workingPool, w)
			// p.ingLock.Unlock()
		}
	}
}

func (p *LWorkerPool) doWork(w *WorkerNode) {
	wid := w.worker.GetWorkerID()
	p.indexLock.Lock()
	w.startTime = time.Now()
	w.status = Working
	ctx, cf := context.WithCancel(p.cancleCtx)
	w.cancleFunc = cf
	p.indexLock.Unlock()
	w.worker.Start(ctx)
	// if err != nil {
	// 	fmt.Println("[ERROR]Start work with error ", err)
	// }

	p.workQuatoChan <- struct{}{}
	p.indexLock.Lock()
	delete(p.workerIndex, wid)
	p.indexLock.Unlock()
}

func (p *LWorkerPool) ForceStop(id string) error {
	return nil
}

func (p *LWorkerPool) ForceStopAll() error {
	return nil
}
