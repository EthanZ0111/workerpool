package workerpool

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

type TestWorker struct {
	id        string
	totalWork int
	finish    int
	level     int
	r         *rand.Rand
}

func NewTestWorker(id string, total, level int) *TestWorker {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &TestWorker{id: id, totalWork: total, level: level, r: r}
}

func (tw *TestWorker) GetWorkerID() string {
	return tw.id
}
func (tw *TestWorker) GetWorkerStatus() (WorkerStatus, error) {
	return WorkerStatus{
		TotalWork: tw.totalWork,
		Finished:  tw.finish,
	}, nil
}

func (tw *TestWorker) Start(ctx context.Context) error {
	for i := 0; i < tw.totalWork; i++ {
		wastTime := time.Duration(tw.r.Int31n(3000)+1) * time.Millisecond
		fmt.Printf("[%s] Work in %s tc %d f %d %v\n", time.Now(), tw.id, tw.totalWork, tw.finish, wastTime)
		time.Sleep(wastTime)
		tw.finish++
	}
	return nil
}

func (tw *TestWorker) GetLevel() int {
	return tw.level
}

func TestWorkerPool(t *testing.T) {
	wp := NewLWorkerPool(10, 1000, false)
	rand.Seed(time.Now().Unix())
	for i := 0; i < 1000; i++ {
		w := NewTestWorker(strconv.Itoa(i), rand.Intn(4)+1, i)
		wp.SignInWork(w)
	}
	for {
		if n, _ := wp.GetAllWork(); n != 0 {
			time.Sleep(time.Second)
			continue
		}
		return
	}
}

func TestList(t *testing.T) {
	l := newCacheList(true)
	recChan := make(chan int, 100000)
	go func() {
		for {
			w := l.pop()
			if w != nil {
				recChan <- w.level
			}
		}
	}()
	go func() {
		for {
			w := l.pop()
			if w != nil {
				recChan <- w.level
			}
		}
	}()
	go func() {
		for {
			w := l.pop()
			if w != nil {
				recChan <- w.level
			}
		}
	}()
	go func() {
		for {
			w := l.pop()
			if w != nil {
				recChan <- w.level
			}
		}
	}()
	go func() {
		for {
			w := l.pop()
			if w != nil {
				recChan <- w.level
			}
		}
	}()
	for i := 0; i < 2000; i++ {
		l.push(&WorkerNode{level: i})
	}
	for i := 2000; i < 4000; i++ {
		l.push(&WorkerNode{level: i})
	}
	tick := time.NewTicker(time.Second * 10)
	set := make(map[int]struct{})
	for {
		select {
		case r := <-recChan:
			set[r] = struct{}{}
			fmt.Println(len(set))
		case <-tick.C:
			fmt.Println(len(l.list), cap(l.list), l.cur)
			return
		}
	}
}

func TestListPopAndPush(t *testing.T) {
	l := newCacheList(false)
	for i := 0; i < 10; i++ {
		l.push(&WorkerNode{level: i})
	}
	for i := 0; i < 6; i++ {
		w := l.pop()
		if w != nil {
			fmt.Println(w.level)
		}
	}
	fmt.Println(l.list[:], len(l.list), cap(l.list), l.cur)
	l.push(&WorkerNode{level: 100})
	fmt.Println(l.list[:], len(l.list), cap(l.list), l.cur)
}
