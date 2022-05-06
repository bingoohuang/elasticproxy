package util

import (
	"context"
	"fmt"
	"log"
	"sync"
)

type FanOut[R any] struct {
	chs     []chan *job[R]
	resultC chan R
}

func NewFanOut[R any](workerNum int) *FanOut[R] {
	chs := make([]chan *job[R], workerNum)
	for i := 0; i < workerNum; i++ {
		chs[i] = make(chan *job[R])
		w := worker[R]{workerNo: i + 1, ch: chs[i]}
		go w.loop()
	}

	return &FanOut[R]{
		chs:     chs,
		resultC: make(chan R),
	}
}

type Job[R any] interface {
	DoJob() R
	GetJobID() string
}

type JobFunc[R any] func() R

func (j JobFunc[R]) DoJob() R         { return j() }
func (j JobFunc[R]) GetJobID() string { return fmt.Sprintf("%+v", j) }

type job[R any] struct {
	context.Context
	Job[R]

	resultC chan R
	wg      *sync.WaitGroup
}

func (f *FanOut[R]) Do(ctx context.Context, value Job[R]) R {
	newCtx, cancelFunc := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(len(f.chs))
	for _, ch := range f.chs {
		ch <- &job[R]{Context: newCtx, Job: value, resultC: f.resultC, wg: &wg}
	}

	result := <-f.resultC
	cancelFunc()
	wg.Wait()

	return result
}

type worker[R any] struct {
	workerNo int
	ch       chan *job[R]
}

func (w *worker[R]) loop() {
	for job := range w.ch {
		w.do(job)
	}
}

func (w *worker[R]) do(job *job[R]) {
	defer job.wg.Done()

	r := job.Job.DoJob()
	select {
	case job.resultC <- r:
		log.Printf("worker %d do job %s sent result %+v successfully", w.workerNo, job.GetJobID(), r)
	case <-job.Context.Done():
		log.Printf("worker %d do job %s caught context done", w.workerNo, job.GetJobID())
	}
}
