package adapters

import (
	"bytes"
	"io"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

// Audio plays short game sounds (e.g. footsteps) through Ebiten's audio context.
type Audio struct {
	ctx     *audio.Context
	stepPCM []byte
}

// NewAudio creates an audio adapter at the given sample rate.
func NewAudio(sampleRate int) *Audio {
	return &Audio{ctx: audio.NewContext(sampleRate)}
}

// LoadStep decodes a RIFF/WAV blob into PCM for repeated playback.
func (a *Audio) LoadStep(wavBytes []byte) {
	if wavBytes == nil {
		a.stepPCM = nil
		return
	}
	st, err := wav.DecodeF32(bytes.NewReader(wavBytes))
	if err != nil {
		a.stepPCM = nil
		return
	}
	pcm, err := io.ReadAll(st)
	if err != nil {
		a.stepPCM = nil
		return
	}
	a.stepPCM = pcm
}

// PlayStep plays the loaded footstep sound, if any.
func (a *Audio) PlayStep() {
	if a.stepPCM == nil {
		return
	}
	a.ctx.NewPlayerF32FromBytes(a.stepPCM).Play()
}
