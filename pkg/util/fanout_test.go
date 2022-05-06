package util

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bingoohuang/gg/pkg/randx"
)

type myJob struct {
	JobID string
}

func (m myJob) DoJob() int {
	time.Sleep(time.Duration(randx.Int64N(100)) * time.Millisecond)
	return randx.Int()
}

func (m myJob) GetJobID() string { return m.JobID }

func TestFanout(t *testing.T) {
	out := NewFanOut[int](10)
	for i := 0; i < 20; i++ {
		result := out.Do(context.Background(), &myJob{JobID: fmt.Sprintf("%d", i+1)})

		t.Log(result)
	}
}

func BenchmarkFanout(b *testing.B) {
	out := NewFanOut[int](10)
	for i := 0; i < b.N; i++ {
		out.Do(context.Background(), JobFunc[int](func() int {
			return randx.Int()
		}))
	}
}
