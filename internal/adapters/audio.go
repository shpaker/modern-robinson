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
	ctx      *audio.Context
	cache    map[string][]byte
	chans    map[int]*audio.Player
	music    string // key of the looping track on the music channel
	volSound float64
	volMusic float64
}

// NewAudio creates an audio adapter at the given sample rate.
func NewAudio(sampleRate int) *Audio {
	return &Audio{
		ctx:      audio.NewContext(sampleRate),
		cache:    map[string][]byte{},
		chans:    map[int]*audio.Player{},
		volSound: 1,
		volMusic: 0.7,
	}
}

// SetVolume sets the effects volume (0..1) for sounds started from now on and
// for anything currently playing.
func (a *Audio) SetVolume(v float64) {
	a.volSound = v
	for ch, p := range a.chans {
		if ch != musicChannel && p != nil {
			p.SetVolume(v)
		}
	}
}

// SetMusicVolume sets the music volume (0..1), applying it immediately.
func (a *Audio) SetMusicVolume(v float64) {
	a.volMusic = v
	if p := a.chans[musicChannel]; p != nil {
		p.SetVolume(v)
	}
}

// Volumes reports the current effects and music volumes.
func (a *Audio) Volumes() (sound, music float64) {
	return a.volSound, a.volMusic
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
	p.SetVolume(a.volSound)
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
	p.SetVolume(a.volMusic)
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
