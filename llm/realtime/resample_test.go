// 职责: 验证流式 PCM16 降采样的连续性、时长和抗混叠。
// 关联文件:
//
//	llm/realtime/resample.go
package realtime

import (
	"encoding/binary"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func tone(hz float64, n int) []byte {
	b := make([]byte, n*2)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(int16(12000*math.Sin(2*math.Pi*hz*float64(i)/24000))))
	}
	return b
}
func rms(b []byte) float64 {
	sum := 0.0
	for i := 200; i < len(b)/2; i++ {
		x := float64(int16(binary.LittleEndian.Uint16(b[i*2:])))
		sum += x * x
	}
	return math.Sqrt(sum / float64(len(b)/2-200))
}
func TestResampleChunkBoundaryAndDuration(t *testing.T) {
	input := tone(1000, 24000)
	whole, err := NewPCM24To16().Append(input)
	require.NoError(t, err)
	stream := NewPCM24To16()
	var chunked []byte
	for i := 0; i < len(input); i += 154 {
		end := min(i+154, len(input))
		b, err := stream.Append(input[i:end])
		require.NoError(t, err)
		chunked = append(chunked, b...)
	}
	require.Equal(t, whole, chunked)
	require.Len(t, whole, 32000)
	require.InDelta(t, 12000/math.Sqrt(2), rms(whole), 100)
}
func TestResampleRejectsAliasingAndBrokenPCM(t *testing.T) {
	a, err := NewPCM24To16().Append(tone(10000, 24000))
	require.NoError(t, err)
	require.Less(t, rms(a), 100.0)
	_, err = NewPCM24To16().Append([]byte{1})
	require.Error(t, err)
}
