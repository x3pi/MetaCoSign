package mvm

/*
#cgo CFLAGS: -w
#cgo CXXFLAGS: -std=c++17 -w
#cgo LDFLAGS: -L./linker/build/lib/static -lmvm_linker -L./c_mvm/build/lib/static -lmvm -lstdc++
#cgo CPPFLAGS: -I./linker/build/include
#include "mvm_linker.hpp"
#include <stdlib.h>
*/
import "C"
import (
	"encoding/hex"
	"unsafe"

	"github.com/meta-node-blockchain/meta-node/pkg/logger"
)

//export GoLogString
func GoLogString(
	flag C.int,
	cString *C.char,
) {
	message := C.GoString(cString)
	switch int(flag) {
	case 0:
		logger.Info(message)
	case 1:
		logger.Debug(message)
	case 2:
		logger.DebugP(message)
	case 3:
		logger.Warn(message)
	case 4:
		logger.Error(message)
	}
}

//export GoLogBytes
func GoLogBytes(
	flag C.int,
	bytes *C.uchar,
	size C.int,
) {
	bMessage := C.GoBytes(unsafe.Pointer(bytes), size)
	hex := hex.EncodeToString(bMessage)
	switch int(flag) {
	case 0:
		logger.Info(hex)
	case 1:
		logger.Debug(hex)
	case 2:
		logger.DebugP(hex)
	case 3:
		logger.Warn(hex)
	case 4:
		logger.Error(hex)
	}
}
