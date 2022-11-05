package uring

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestWritev(t *testing.T) {
	f, err := ioutil.TempFile("", "writev-tests-")
	require.NoError(t, err)
	defer f.Close()

	ring, err := Setup(4, nil)
	require.NoError(t, err)
	defer ring.Close()

	var offset uint64
	bufs := [4][8]byte{}
	vectors := [4][]syscall.Iovec{}

	for round := 0; round < 10; round++ {
		for i := 0; i < 4; i++ {
			buf := bufs[i]
			_, _ = rand.Read(buf[:])
			bufs[i] = buf
			vectors[i] = []syscall.Iovec{
				{
					Base: &buf[0],
					Len:  uint64(len(buf)),
				},
			}
			sqe := ring.GetSQEntry()
			Writev(sqe, f.Fd(), vectors[i], offset, 0)
			offset += uint64(len(buf))
		}

		_, err = ring.Submit(4, nil)
		require.NoError(t, err)

		for i := 0; i < 4; i++ {
			cqe, err := ring.GetCQEntry(0)
			require.NoError(t, err)
			require.True(t, cqe.Result() >= 0, "failed with %v", syscall.Errno(-cqe.Result()))
		}

		buf := [8]byte{}
		for i := 0; i < 4; i++ {
			n, err := f.Read(buf[:])
			require.NoError(t, err)
			require.Equal(t, len(buf), n)
			require.Equal(t, bufs[i], buf)
		}
	}
}

func TestReadv(t *testing.T) {
	f, err := ioutil.TempFile("", "readv-tests-")
	require.NoError(t, err)
	defer f.Close()

	ring, err := Setup(4, nil)
	require.NoError(t, err)
	defer ring.Close()

	var offset uint64
	const num = 3
	bufs := [num][8]byte{}
	vectors := [num][]syscall.Iovec{}

	for round := 0; round < 10; round++ {

		wbuf := [num * 8]byte{}

		_, _ = rand.Read(wbuf[:])
		n, err := f.Write(wbuf[:])
		require.NoError(t, err)
		require.Equal(t, len(wbuf), n)

		for i := 0; i < num; i++ {
			sqe := ring.GetSQEntry()
			vectors[i] = []syscall.Iovec{
				{
					Base: &bufs[i][0],
					Len:  uint64(len(bufs[i])),
				},
			}
			Readv(sqe, f.Fd(), vectors[i], offset, 0)
			offset += uint64(len(bufs[i]))
		}

		_, err = ring.Submit(num, nil)
		require.NoError(t, err)

		for i := 0; i < num; i++ {
			cqe, err := ring.GetCQEntry(0)
			require.NoError(t, err)
			require.Equal(t, len(bufs[i]), int(cqe.Result()), "failed with %v", syscall.Errno(-cqe.Result()))
			require.Equal(t, wbuf[i*8:(i+1)*8], bufs[i][:])
		}
	}
}

func TestCopy(t *testing.T) {
	from, err := ioutil.TempFile("", "copy-from-")
	require.NoError(t, err)
	defer from.Close()

	to, err := ioutil.TempFile("", "copy-to-")
	require.NoError(t, err)
	defer to.Close()

	ring, err := Setup(4, nil)
	require.NoError(t, err)
	defer ring.Close()

	buf := make([]byte, 4096)
	_, _ = rand.Read(buf)
	_, err = from.Write(buf)
	require.NoError(t, err)
	off, err := from.Seek(0, 0)
	require.NoError(t, err)
	require.Equal(t, int64(0), off)

	reuse := [32]byte{}
	rlth := uint64(len(reuse))
	vector := []syscall.Iovec{
		{
			Base: &reuse[0],
			Len:  rlth,
		},
	}
	var (
		offset uint64
	)
	for {
		read := ring.GetSQEntry()
		write := ring.GetSQEntry()

		Readv(read, from.Fd(), vector, offset, 0)
		read.SetFlags(IOSQE_IO_LINK)
		Writev(write, to.Fd(), vector, offset, 0)

		_, err := ring.Submit(2)
		require.NoError(t, err)

		rcqe, err := ring.GetCQEntry(0)
		require.NoError(t, err)
		require.True(t, rcqe.Result() >= 0, "read result %d ('%v')", rcqe.Result(), syscall.Errno(-rcqe.Result()))

		ret := rcqe.Result()
		if ret == 0 {
			break
		}

		wcqe, err := ring.GetCQEntry(0)
		require.NoError(t, err)
		require.Equal(t, ret, wcqe.Result(), "write result %d ('%v')", wcqe.Result(), syscall.Errno(-wcqe.Result()))

		offset += rlth
	}

	fromData, err := ioutil.ReadAll(from)
	toData, err := ioutil.ReadAll(to)
	require.NoError(t, err, "failed to read 'from'")
	require.NoError(t, err, "failed to read 'to'")
	require.Equal(t, len(fromData), len(toData))
	require.Equal(t, fromData, toData)
}

func TestReuseSQEntries(t *testing.T) {
	ring, err := Setup(2, nil)
	require.NoError(t, err)

	for r := 0; r < 10; r++ {
		for i := 1; i <= 2; i++ {
			sqe := ring.GetSQEntry()
			sqe.Reset()
			require.Equal(t, uint64(0), sqe.userData)
			Nop(sqe)
			sqe.SetUserData(uint64(i))
		}
		n, err := ring.Submit(2)
		require.NoError(t, err)
		require.Equal(t, uint32(2), n)

		for i := 1; i <= 2; i++ {
			cqe, err := ring.GetCQEntry(0)
			require.NoError(t, err)
			require.Equal(t, uint64(i), cqe.UserData())
		}
	}

}

func TestNoEnter(t *testing.T) {
	ring, err := Setup(4, nil)
	require.NoError(t, err)
	defer ring.Close()

	sqe := ring.GetSQEntry()
	Nop(sqe)
	_, err = ring.Submit(0)
	require.NoError(t, err)

	start := time.Now()
	for time.Since(start) < time.Second {
		_, err := ring.GetCQEntry(0)
		if err == nil {
			return
		}
	}
	require.FailNow(t, "nop operation wasn't completed")
}

func TestResubmitBeforeCompletion(t *testing.T) {
	n := 2048
	ring, err := Setup(uint(n), nil)
	require.NoError(t, err)
	defer ring.Close()

	for round := 0; round < 2; round++ {
		// sq entry can be reused after call to Submit returned
		for i := uint64(1); i <= uint64(n); i++ {
			sqe := ring.GetSQEntry()
			Nop(sqe)
			sqe.SetUserData(i)
		}

		_, err = ring.Submit(0)
		require.NoError(t, err)
	}
	for round := 0; round < 2; round++ {
		for i := uint64(1); i <= uint64(n); i++ {
			for {
				cqe, err := ring.GetCQEntry(0)
				if err != nil {
					continue
				}
				require.Equal(t, i, cqe.UserData())
				break
			}
		}
	}
}

func TestReadWriteFixed(t *testing.T) {
	ring, err := Setup(32, nil)
	require.NoError(t, err)
	defer ring.Close()

	f, err := ioutil.TempFile("", "test")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	data := []byte("ping")
	resp := make([]byte, len(data))
	iovec := []unix.Iovec{
		{
			Base: &data[0],
			Len:  uint64(len(data)),
		},
		{
			Base: &resp[0],
			Len:  uint64(len(data)),
		},
	}

	require.NoError(t, ring.RegisterBuffers(iovec))

	sqe := ring.GetSQEntry()
	WriteFixed(sqe, f.Fd(), iovec[0].Base, iovec[0].Len, 0, 0, 0)
	_, err = ring.Submit(1)
	require.NoError(t, err)

	cqe, err := ring.GetCQEntry(1)
	require.NoError(t, err)
	require.Equal(t, int32(len(data)), cqe.Result(), syscall.Errno(-cqe.Result()))

	out := make([]byte, len(data))
	_, err = f.ReadAt(out, 0)
	require.NoError(t, err)
	require.Equal(t, data, out)

	in := []byte("pong")
	_, err = f.WriteAt(in, 0)
	require.NoError(t, err)

	sqe = ring.GetSQEntry()
	ReadFixed(sqe, f.Fd(), iovec[1].Base, iovec[1].Len, 0, 0, 1)
	_, err = ring.Submit(1)
	require.NoError(t, err)

	cqe, err = ring.GetCQEntry(1)
	require.NoError(t, err)
	require.Equal(t, int32(len(data)), cqe.Result(), syscall.Errno(-cqe.Result()))

	require.Equal(t, in, resp)
}

func TestIOPoll(t *testing.T) {
	f, err := ioutil.TempFile("/home/rocky/GolandProjects/awesomeProject", "tmp.txt")
	require.NoError(t, err)
	defer f.Close()
	ring, err := Setup(4, &IOUringParams{Flags: IORING_SETUP_SQPOLL})
	require.NoError(t, err)
	defer ring.Close()

	//b, err := unix.ByteSliceFromString("")

	pathDir, err := os.Open("/home/rocky/GolandProjects/awesomeProject/tmp.txt")
	//if err != nil {
	//	fmt.Printf("open directory error: %v\n", err)
	//	return
	//}
	//defer pathDir.Close()

	in := make([]byte, 100)
	sqe := ring.GetSQEntry()
	Read(sqe, pathDir.Fd(), in)
	if err != nil {
		fmt.Printf("prep request error: %v\n", err)
		return
	}

	//bp := unsafe.Pointer(&b[0])

	// returns immediatly
	_, err = ring.Submit(1, nil)
	//_, err = ring.GetCQEntry(0)
	//require.Error(t, syscall.EAGAIN, err)

	// returns once consumed scheduler time slice
	//while(ring.cq.cqes.get())
	var CQ CQEntry
	for {
		CQ, err = ring.GetCQEntry(1)
		if err == nil {
			break
		}
	}
	result := CQ.Result()
	fmt.Printf(string(result))
	fmt.Printf(string(in))
	require.Error(t, syscall.EAGAIN, err)
}

func TestAccept(t *testing.T) {
	f, err := ioutil.TempFile("/home/rocky/GolandProjects/awesomeProject", "tmp.txt")
	require.NoError(t, err)
	defer f.Close()
	ring, err := Setup(4, &IOUringParams{Flags: IORING_SETUP_SQPOLL})
	//ring, err := Setup(4, nil)
	require.NoError(t, err)
	defer ring.Close()

	//b, err := unix.ByteSliceFromString("")

	//pathDir, err := os.Open("/home/rocky/GolandProjects/awesomeProject/tmp.txt")
	//if err != nil {
	//	fmt.Printf("open directory error: %v\n", err)
	//	return
	//}
	//defer pathDir.Close()
	fd := listenSocket("127.0.0.1:50001")
	//in := make([]byte, 100)
	in2 := make([]byte, 100)
	sqe := ring.GetSQEntry()
	//Read(sqe, pathDir.Fd(), in)
	Accept(sqe, uintptr(fd), in2)
	fmt.Println(sqe.userData)

	if err != nil {
		fmt.Printf("prep request error: %v\n", err)
		return
	}

	//bp := unsafe.Pointer(&b[0])

	// returns immediatly
	_, err = ring.Submit(1)
	//_, err = ring.GetCQEntry(0)
	//require.Error(t, syscall.EAGAIN, err)

	// returns once consumed scheduler time slice
	//while(ring.cq.cqes.get())
	var CQ CQEntry
	for {
		CQ, err = ring.GetCQEntry(1)
		if err == nil {
			break
		}
	}
	//temp := interface{}(int(CQ.res))
	//addr := temp.(*syscall.SockaddrInet4)
	//clientAddr := fmt.Sprintf("%s:%d", net.IPv4(addr.Addr[0], addr.Addr[1], addr.Addr[2], addr.Addr[3]), addr.Port)
	//fmt.Printf("Client Conn: %s\n", clientAddr)
	result := CQ.Result()
	fmt.Println(string(result))
	fmt.Println(string(in2))
	require.Error(t, syscall.EAGAIN, err)

}

func listenSocket(addr string) int {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		panic(err)
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		panic(err)
	}

	sockaddr := &syscall.SockaddrInet4{Port: tcpAddr.Port}
	copy(sockaddr.Addr[:], tcpAddr.IP.To4())
	if err := syscall.Bind(fd, sockaddr); err != nil {
		panic(err)
	}

	if err := syscall.Listen(fd, syscall.SOMAXCONN); err != nil {
		panic(err)
	}
	return fd
}
