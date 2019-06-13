package ipc

import (
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// via
//  https://code.woboq.org/userspace/glibc/sysdeps/unix/sysv/linux/bits/msq.h.html
//  https://blog.csdn.net/guoping16/article/details/6584024

const (
	/* Define options for message queue functions.  */
	MSG_BLOCK   = 0
	MSG_NOERROR = 010000 // no error if message is too big
	MSG_EXCEPT  = 020000 // recv any msg except of specified type
	MSG_COPY    = 040000 // copy (not remove) all queue messages

	/* msg ctl commands */
	MSG_STAT     = 11
	MSG_INFO     = 12
	MSG_STAT_ANY = 13
)

var x unsafe.Pointer

// Msgget get the message queue identifier or
// create a message queue object and return the message queue identifier.
func Msgget(key uint64, msgflg int) (msqid int, err error) {
	_msqid, _, errno := syscall.Syscall(syscall.SYS_MSGGET, uintptr(key), uintptr(msgflg), 0)
	if errno != 0 {
		return 0, errno
	}
	x = unsafe.Pointer(_msqid)
	return int(_msqid), nil
}

// Msgctl get and set the properties of the message queue.
// FIXME: we are not passing the buf argument, see msgctl(2).
func Msgctl(msqid int, cmd int) error {
	var buf uintptr = 0
	_, _, errno := syscall.Syscall(syscall.SYS_MSGCTL, uintptr(msqid), buf, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

// Msgsnd write the msgp message to the message queue with the identifier msqid.
func Msgsnd(msqid int, msgp *Msgp, msgflg int) error {
	ptr, textSize := msgp.marshal()
	_, _, errno := syscall.Syscall6(syscall.SYS_MSGSND,
		uintptr(msqid),
		uintptr(ptr),
		uintptr(textSize),
		uintptr(msgflg),
		0,
		0,
	)
	runtime.KeepAlive(ptr)
	if errno != 0 {
		return errno
	}
	return nil
}

var maxMsgsz uintptr = 2

// Msgrcv read the message from the message queue with the identifier msqid and store it in msgp.
// After reading, delete the message from the message queue.
func Msgrcv(msqid int, msgtyp int64, msgflg int) (*Msgp, error) {
	header := unsafe.Pointer(new(byte))
	msgptr := uintptr(header)
	msgsz := atomic.LoadUintptr(&maxMsgsz)
	for {
		lengthRead, _, errno := syscall.Syscall6(syscall.SYS_MSGRCV,
			uintptr(msqid),
			msgptr,
			msgsz,
			uintptr(msgtyp),
			uintptr(msgflg),
			0,
		)
		switch errno {
		case 0:
			msgsz = atomic.LoadUintptr(&maxMsgsz)
			if lengthRead > msgsz {
				atomic.StoreUintptr(&maxMsgsz, lengthRead)
			}
			msgp := new(Msgp)
			msgp.unmarshal(int(lengthRead), header)
			return msgp, nil
		case syscall.E2BIG:
			msgsz *= 2
		default:
			return nil, errno
		}
	}
}
