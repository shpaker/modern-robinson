package adapters

import (
	"bytes"
	"io"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

// Audio plays game sounds on numbered channels (1-9), mirroring the engine:
// re-triggering a channel stops its current sound. Decoded PCM is cached by key.
type Audio struct {
	ctx   *audio.Context
	cache map[string][]byte
	chans map[int]*audio.Player
	music string // key of the looping track on the music channel
}

// NewAudio creates an audio adapter at the given sample rate.
func NewAudio(sampleRate int) *Audio {
	return &Audio{
		ctx:   audio.NewContext(sampleRate),
		cache: map[string][]byte{},
		chans: map[int]*audio.Player{},
	}
}

func (a *Audio) decode(key string, wavBytes []byte) []byte {
	if pcm, ok := a.cache[key]; ok {
		return pcm
	}
	var pcm []byte
	if wavBytes != nil {
		if st, err := wav.DecodeF32(bytes.NewReader(wavBytes)); err == nil {
			pcm, _ = io.ReadAll(st)
		}
	}
	a.cache[key] = pcm
	return pcm
}

// Play decodes wavBytes (cached under key) and plays it on the given channel,
// stopping whatever that channel was playing.
func (a *Audio) Play(key string, wavBytes []byte, channel int) {
	pcm := a.decode(key, wavBytes)
	if pcm == nil {
		return
	}
	if old := a.chans[channel]; old != nil {
		old.Pause()
	}
	p := a.ctx.NewPlayerF32FromBytes(pcm)
	a.chans[channel] = p
	p.Play()
}

// musicChannel is a reserved channel for the looping background track.
const musicChannel = 0

// PlayMusic loops a track on the music channel until replaced or stopped.
// Re-triggering the same key keeps the current playback.
func (a *Audio) PlayMusic(key string, wavBytes []byte) {
	if a.music == key {
		return
	}
	pcm := a.decode(key, wavBytes)
	if pcm == nil {
		return
	}
	a.StopMusic()
	loop := audio.NewInfiniteLoopF32(bytes.NewReader(pcm), int64(len(pcm)))
	p, err := a.ctx.NewPlayerF32(loop)
	if err != nil {
		return
	}
	a.music = key
	a.chans[musicChannel] = p
	p.Play()
}

// StopMusic silences the music channel.
func (a *Audio) StopMusic() {
	if old := a.chans[musicChannel]; old != nil {
		old.Pause()
	}
	a.music = ""
}
