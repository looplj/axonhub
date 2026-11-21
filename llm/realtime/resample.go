// 职责: 用连续双相低通 FIR 将单声道 24 kHz PCM16 降采样到 16 kHz。
// 关联文件:
//
//	llm/realtime/adapter.go
//
// 注意事项:
//
//	相位与 63 点历史跨块保持；Blackman 窗低通抑制混叠，固定约 1.3ms 群延迟。
package realtime

import (
	"encoding/binary"
	"errors"
	"math"
)

const firTaps = 63

type PCM24To16 struct {
	history     [firTaps]float64
	coeff       [2][firTaps]float64
	count, next int64
}

func NewPCM24To16() *PCM24To16 {
	r := &PCM24To16{}
	for phase := 0; phase < 2; phase++ {
		sum := 0.0
		for k := 0; k < firTaps; k++ {
			x := float64(k) + float64(phase)/2 - 31
			h := 0.6
			if x != 0 {
				h = math.Sin(0.6*math.Pi*x) / (math.Pi * x)
			}
			window := 0.42 - 0.5*math.Cos(2*math.Pi*float64(k)/62) + 0.08*math.Cos(4*math.Pi*float64(k)/62)
			r.coeff[phase][k] = h * window
			sum += r.coeff[phase][k]
		}
		for k := 0; k < firTaps; k++ {
			r.coeff[phase][k] /= sum
		}
	}
	return r
}
func (r *PCM24To16) Append(pcm []byte) ([]byte, error) {
	if len(pcm)%2 != 0 {
		return nil, errors.New("PCM16 chunks must contain complete samples")
	}
	out := make([]byte, 0, len(pcm)*2/3+4)
	for offset := 0; offset < len(pcm); offset += 2 {
		index := r.count
		r.history[index%firTaps] = float64(int16(binary.LittleEndian.Uint16(pcm[offset:])))
		r.count++
		for r.next/2 == index {
			phase := int(r.next % 2)
			sample := 0.0
			for k := 0; k < firTaps; k++ {
				if index-int64(k) >= 0 {
					sample += r.history[(index-int64(k))%firTaps] * r.coeff[phase][k]
				}
			}
			value := int16(math.Round(max(-32768.0, min(32767.0, sample))))
			out = binary.LittleEndian.AppendUint16(out, uint16(value))
			r.next += 3
		}
	}
	return out, nil
}
