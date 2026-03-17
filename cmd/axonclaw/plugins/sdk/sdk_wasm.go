//go:build wasip1 && wasm

package sdk

import (
	"encoding/json"
	"fmt"
	"runtime"
	"unsafe"
)

//go:wasmimport env host_input_len
func hostInputLen() uint32

//go:wasmimport env host_input_copy
func hostInputCopy(ptr uint32, size uint32)

//go:wasmimport env host_output_reset
func hostOutputReset()

//go:wasmimport env host_output_append
func hostOutputAppend(ptr uint32, size uint32)

//go:wasmimport env host_log
func hostLog(level uint32, ptr uint32, size uint32)

type ErrorResponse struct {
	Error string `json:"error"`
}

func Input() []byte {
	size := hostInputLen()
	if size == 0 {
		return nil
	}

	buf := make([]byte, size)
	hostInputCopy(pointer(buf), uint32(len(buf)))
	runtime.KeepAlive(buf)

	return buf
}

func Output(data []byte) {
	hostOutputReset()
	if len(data) == 0 {
		return
	}

	hostOutputAppend(pointer(data), uint32(len(data)))
	runtime.KeepAlive(data)
}

func Log(level uint32, message string) {
	data := []byte(message)
	if len(data) == 0 {
		return
	}
	hostLog(level, pointer(data), uint32(len(data)))
	runtime.KeepAlive(data)
}

func RunJSON[I any, O any](fn func(I) (O, error)) {
	var input I
	if data := Input(); len(data) > 0 {
		if err := json.Unmarshal(data, &input); err != nil {
			writeError(fmt.Errorf("decode input: %w", err))
			return
		}
	}

	output, err := fn(input)
	if err != nil {
		writeError(err)
		return
	}

	data, err := json.Marshal(output)
	if err != nil {
		writeError(fmt.Errorf("encode output: %w", err))
		return
	}

	Output(data)
}

func writeError(err error) {
	data, marshalErr := json.Marshal(ErrorResponse{Error: err.Error()})
	if marshalErr != nil {
		data = []byte(`{"error":"plugin failure"}`)
	}
	Output(data)
}

func pointer(data []byte) uint32 {
	if len(data) == 0 {
		return 0
	}

	return uint32(uintptr(unsafe.Pointer(unsafe.SliceData(data))))
}
