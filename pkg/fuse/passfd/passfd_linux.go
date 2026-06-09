//go:build linux

package passfd

import "syscall"

const msgCmsgCloexec = syscall.MSG_CMSG_CLOEXEC
