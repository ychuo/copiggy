//go:build darwin

// Package objc contains the Objective-C runtime primitives shared by native
// adapters. It does not initialize input monitoring or request permissions.
package objc

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var runtimeState struct {
	once        sync.Once
	err         error
	messageSend uintptr
	poolPush    func() uintptr
	poolPop     func(uintptr)
}

// Init loads only libobjc and Foundation. Feature adapters load their own
// frameworks and symbols so a guard initialization failure cannot disable UI.
func Init() error {
	runtimeState.once.Do(func() {
		defer func() {
			if failure := recover(); failure != nil {
				runtimeState.err = fmt.Errorf("Objective-C runtime: %v", failure)
			}
		}()
		handle, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			runtimeState.err = err
			return
		}
		if _, err = purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
			runtimeState.err = err
			return
		}
		runtimeState.messageSend, err = purego.Dlsym(handle, "objc_msgSend")
		if err != nil {
			runtimeState.err = err
			return
		}
		purego.RegisterLibFunc(&runtimeState.poolPush, handle, "objc_autoreleasePoolPush")
		purego.RegisterLibFunc(&runtimeState.poolPop, handle, "objc_autoreleasePoolPop")
	})
	return runtimeState.err
}

// MessageSend is available after successful Init, for typed ABI bindings.
func MessageSend() uintptr { return runtimeState.messageSend }

func Send(object uintptr, selector objc.SEL, arguments ...uintptr) uintptr {
	if object == 0 {
		return 0
	}
	if len(arguments) == 0 {
		value, _, _ := purego.SyscallN(runtimeState.messageSend, object, uintptr(selector))
		return value
	}
	args := make([]uintptr, 2, len(arguments)+2)
	args[0], args[1] = object, uintptr(selector)
	args = append(args, arguments...)
	value, _, _ := purego.SyscallN(runtimeState.messageSend, args...)
	return value
}

func String(object uintptr) string {
	length := Send(object, objc.RegisterName("lengthOfBytesUsingEncoding:"), 4)
	if length == 0 || length > 4096 {
		return ""
	}
	address := Send(object, objc.RegisterName("UTF8String"))
	if address == 0 {
		return ""
	}
	// Cocoa owns this memory; convert to a Go string before its owner or pool
	// is released. Match the pointer-bitcast idiom in MyGo's Darwin adapter.
	return string(unsafe.Slice(*(**byte)(unsafe.Pointer(&address)), int(length)))
}

func WithPool(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := runtimeState.poolPush()
	defer runtimeState.poolPop(pool)
	fn()
}
