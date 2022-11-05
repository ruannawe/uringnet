package fixed

import (
	"encoding/binary"
	"io/ioutil"
	"os"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"uring"
	"uring/loop"
)

func TestWrite(t *testing.T) {
	l, err := loop.Setup(1024, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })

	f, err := ioutil.TempFile("", "test")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	n := 100
	size := 10
	pool, err := New(l, size, n)
	require.NoError(t, err)

	run := func() {
		for i := 0; i < n; i++ {
			buf := pool.Get()
			defer pool.Put(buf)
			cqe, err := l.Syscall(func(sqe *uring.SQEntry) {
				uring.WriteFixed(sqe, f.Fd(), buf.Base(), buf.Len(), 0, 0, buf.Index())
			})
			require.NoError(t, err)
			require.Equal(t, int32(10), cqe.Result(), syscall.Errno(-cqe.Result()))
		}
	}
	// run it couple of times to test that buffers are reused correctly
	for i := 0; i < 3; i++ {
		run()
	}
}

func TestConcurrentWrites(t *testing.T) {
	l, err := loop.Setup(1024, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })

	f, err := ioutil.TempFile("", "test-concurrent-writes-")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	var wg sync.WaitGroup
	var n int64 = 10000

	pool, err := New(l, 8, int(n))
	require.NoError(t, err)
	for i := int64(0); i < n; i++ {
		wg.Add(1)
		go func(i uint64) {
			buf := pool.Get()
			defer pool.Put(buf)
			binary.BigEndian.PutUint64(buf.Bytes(), i)
			_, _ = l.Syscall(func(sqe *uring.SQEntry) {
				uring.WriteFixed(sqe, f.Fd(), buf.Base(), buf.Len(), i*8, 0, buf.Index())
			})
			wg.Done()
		}(uint64(i))
	}
	wg.Wait()

	buf2 := make([]byte, 8)
	for i := int64(0); i < n; i++ {
		_, err := f.ReadAt(buf2, i*8)
		require.NoError(t, err)
		rst := binary.BigEndian.Uint64(buf2[:])
		require.Equal(t, i, int64(rst))
	}
}

func BenchmarkPool(b *testing.B) {
	l, err := loop.Setup(1024, nil, nil)
	require.NoError(b, err)
	b.Cleanup(func() { l.Close() })

	pool, err := New(l, 8, 50000)
	require.NoError(b, err)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			buf.B[0] = 10
			pool.Put(buf)
		}
	})
}

func BenchmarkSyncPool(b *testing.B) {
	pool := sync.Pool{
		New: func() interface{} {
			return make([]byte, 8)
		},
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get().([]byte)
			buf[0] = 10
			pool.Put(buf)
		}
	})
}
