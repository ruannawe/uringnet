package uring

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Nop ...
func Nop(sqe *SQEntry) {
	sqe.opcode = IORING_OP_NOP
}

// Write ...
func Write(sqe *SQEntry, fd uintptr, buf []byte) {
	sqe.opcode = IORING_OP_WRITE
	sqe.fd = int32(fd)
	sqe.addr = (uint64)(uintptr(unsafe.Pointer(&buf[0])))
	sqe.len = uint32(len(buf))
}

// Read ...
func Read(sqe *SQEntry, fd uintptr, buf []byte) {
	sqe.opcode = IORING_OP_READ
	sqe.fd = int32(fd)
	sqe.addr = (uint64)(uintptr(unsafe.Pointer(&buf[0])))
	sqe.len = uint32(len(buf))
}

func ReadNoBuf(sqe *SQEntry, fd uintptr, length uint32) {
	sqe.opcode = IORING_OP_READ
	sqe.fd = int32(fd)
	//sqe.addr = (uint64)(uintptr(unsafe.Pointer(&buf[0])))
	sqe.len = length
}

// Writev ...
func Writev(sqe *SQEntry, fd uintptr, iovec []syscall.Iovec, offset uint64, flags uint32) {
	sqe.opcode = IORING_OP_WRITEV
	sqe.fd = int32(fd)
	sqe.len = uint32(len(iovec))
	sqe.offset = offset
	sqe.opcodeFlags = flags
	sqe.addr = (uint64)(uintptr(unsafe.Pointer(&iovec[0])))
}

// Readv
func Readv(sqe *SQEntry, fd uintptr, iovec []syscall.Iovec, offset uint64, flags uint32) {
	sqe.opcode = IORING_OP_READV
	sqe.fd = int32(fd)
	sqe.len = uint32(len(iovec))
	sqe.offset = offset
	sqe.opcodeFlags = flags
	sqe.addr = (uint64)(uintptr(unsafe.Pointer(&iovec[0])))
}

// WriteFixed ...
func WriteFixed(sqe *SQEntry, fd uintptr, base *byte, len, offset uint64, flags uint32, bufIndex uint16) {
	sqe.opcode = IORING_OP_WRITE_FIXED
	sqe.fd = int32(fd)
	sqe.len = uint32(len)
	sqe.offset = offset
	sqe.opcodeFlags = flags
	sqe.addr = (uint64)(uintptr(unsafe.Pointer(base)))
	sqe.SetBufIndex(bufIndex)
}

// ReadFixed ...
func ReadFixed(sqe *SQEntry, fd uintptr, base *byte, len, offset uint64, flags uint32, bufIndex uint16) {
	sqe.opcode = IORING_OP_READ_FIXED
	sqe.fd = int32(fd)
	sqe.len = uint32(len)
	sqe.offset = offset
	sqe.opcodeFlags = flags
	sqe.addr = (uint64)(uintptr(unsafe.Pointer(base)))
	sqe.SetBufIndex(bufIndex)
}

// Fsync ...
func Fsync(sqe *SQEntry, fd uintptr) {
	sqe.opcode = IORING_OP_FSYNC
	sqe.fd = int32(fd)
}

// Fdatasync ...
func Fdatasync(sqe *SQEntry, fd uintptr) {
	sqe.opcode = IORING_OP_FSYNC
	sqe.fd = int32(fd)
	sqe.opcodeFlags = IORING_FSYNC_DATASYNC
}

// Openat
func Openat(sqe *SQEntry, dfd int32, pathptr *byte, flags uint32, mode uint32) {
	sqe.opcode = IORING_OP_OPENAT
	sqe.fd = dfd
	sqe.opcodeFlags = flags
	sqe.addr = (uint64)(uintptr(unsafe.Pointer(pathptr)))
	sqe.len = mode
}

// Close ...
func Close(sqe *SQEntry, fd uintptr) {
	sqe.opcode = IORING_OP_CLOSE
	sqe.fd = int32(fd)
}

// Send ...
func Send(sqe *SQEntry, fd uintptr, buf []byte, flags uint32) {
	sqe.SetOpcode(IORING_OP_SEND)
	sqe.SetFD(int32(fd))
	sqe.SetAddr((uint64)(uintptr(unsafe.Pointer(&buf[0]))))
	sqe.SetLen(uint32(len(buf)))
	sqe.SetOpcodeFlags(flags)
}

// Recv ...
func Recv(sqe *SQEntry, fd uintptr, buf []byte, flags uint32) {
	sqe.SetOpcode(IORING_OP_RECV)
	sqe.SetFD(int32(fd))
	sqe.SetAddr((uint64)(uintptr(unsafe.Pointer(&buf[0]))))
	sqe.SetLen(uint32(len(buf)))
	sqe.SetOpcodeFlags(flags)
}

// Timeout operation.
// if abs is true then IORING_TIMEOUT_ABS will be added to timeoutFlags.
// count is the number of events to wait.
func Timeout(sqe *SQEntry, ts *unix.Timespec, abs bool, count uint64) {
	sqe.SetFD(-1)
	sqe.SetOpcode(IORING_OP_TIMEOUT)
	sqe.SetAddr((uint64)(uintptr(unsafe.Pointer(ts))))
	sqe.SetLen(1)
	if abs {
		sqe.SetOpcodeFlags(IORING_TIMEOUT_ABS)
	}
	sqe.SetOffset(count)
}

// LinkTimeout will cancel linked operation if it doesn't complete in time.
func LinkTimeout(sqe *SQEntry, ts *unix.Timespec, abs bool) {
	sqe.SetFD(-1)
	sqe.SetOpcode(IORING_OP_LINK_TIMEOUT)
	sqe.SetAddr((uint64)(uintptr(unsafe.Pointer(ts))))
	sqe.SetLen(1)
	if abs {
		sqe.SetOpcodeFlags(IORING_TIMEOUT_ABS)
	}
}

// Read ...
func Mkdirat(sqe *SQEntry, fd uintptr, buf []byte) {
	sqe.opcode = IORING_OP_MKDIRAT
	sqe.fd = int32(fd)
	sqe.addr = (uint64)(uintptr(unsafe.Pointer(&buf[0])))
	sqe.len = uint32(len(buf))
}

// Read ...
func Accept(sqe *SQEntry, fd uintptr, clientAddr *syscall.RawSockaddrAny, len *uint32) {
	//var rsa unix.RawSockaddrAny

	sqe.SetOpcode(IORING_OP_ACCEPT)
	sqe.fd = int32(fd)

	sqe.SetAddr(uint64(uintptr(unsafe.Pointer(clientAddr))))

	sqe.len = 0
	sqe.SetOffset(uint64(uintptr(unsafe.Pointer(&len))))
}

func ProvideBuf(sqe *SQEntry, clientAddr [][1024]byte, bufferCount uint32, bufferSize uint32, gid uint16) {

	sqe.SetOpcode(IORING_OP_PROVIDE_BUFFERS)
	//set the buffer group id
	sqe.SetBufGroup(gid)
	//how many user buffers in the buffer group
	sqe.fd = int32(bufferCount)
	//the size of one buffer item in the buffer group
	sqe.len = bufferSize
	//user buffer's first index
	sqe.SetOffset(0) // = uint64(uintptr(unsafe.Pointer(&len)))

	sqe.SetAddr((uint64)(uintptr(unsafe.Pointer(&clientAddr[0]))))

}

func ProvideSingleBuf(sqe *SQEntry, clientAddr *[1024]byte, bufferCount uint32, bufferSize uint32, gid uint16, offset uint64) {

	sqe.SetOpcode(IORING_OP_PROVIDE_BUFFERS)
	//set the buffer group id
	sqe.SetBufGroup(gid)
	//how many user buffers in the buffer group
	sqe.fd = int32(bufferCount)
	//the size of one buffer item in the buffer group
	sqe.len = bufferSize
	//user buffer's first index
	sqe.SetOffset(offset) // = uint64(uintptr(unsafe.Pointer(&len)))

	sqe.SetAddr((uint64)(uintptr(unsafe.Pointer(clientAddr))))

}
