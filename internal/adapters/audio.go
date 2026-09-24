package adapters

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/shpaker/modern-robinson/internal/interfaces"
)

// Audio plays game sounds the two ways the engine does: a named file on a
// numbered channel (1-9), where re-triggering the channel stops its current
// sound, and a scene's sound variable through its own set of voices. Decoded
// PCM is cached by key.
type Audio struct {
	ctx   *audio.Context
	cache map[string][]byte
	chans map[int]*audio.Player
	// voices holds each sound variable's buffers by key, so a re-trigger can
	// find what is still sounding and a scene change can cut it all.
	voices   map[string]*voicePool
	music    string // key of the looping track on the music channel
	volSound float64
	volMusic float64
}

// voice is one buffer of a sound variable: the player sounding on it and the
// level it was started at, so the volume slider can rescale it.
type voice struct {
	p     *audio.Player
	scale float64
}

// voicePool is a sound variable's buffers: SoundVariables' third field says
// how many copies of the sound may play at once (step,"step.wav",1 is one;
// the ambient entries carry 5 or 7).
type voicePool struct {
	voices []voice
	cur    int
}

// pick chooses the voice a new trigger sounds on, the engine's way (0x422dd0):
// the current one while it is quiet; else, with more than one, the next in
// turn, cut and restarted if it is still sounding; else none — a lone buffer
// that is still playing drops the trigger, so SCENA0's surf (6.8 s, re-fired
// every 2.3 s) plays through instead of restarting. busy reports whether a
// voice is sounding.
func (vp *voicePool) pick(busy func(i int) bool) int {
	if !busy(vp.cur) {
		return vp.cur
	}
	if len(vp.voices) == 1 {
		return -1
	}
	vp.cur = (vp.cur + 1) % len(vp.voices)
	return vp.cur
}

var _ interfaces.IAudio = (*Audio)(nil)

// NewAudio creates an audio adapter at the given sample rate.
func NewAudio(sampleRate int) *Audio {
	return &Audio{
		ctx:      audio.NewContext(sampleRate),
		cache:    map[string][]byte{},
		chans:    map[int]*audio.Player{},
		voices:   map[string]*voicePool{},
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
	for _, vp := range a.voices {
		for _, vc := range vp.voices {
			if vc.p != nil {
				vc.p.SetVolume(v * vc.scale)
			}
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

// PlayVoice sounds a sound variable through its own voices (see voicePool):
// the channel a Sound event names plays no part for a variable, only for a
// quoted file (0x41B4D4). volScale attenuates the shot and pan places it left
// (-1) to right (+1) — the ambient pool rolls both; a script's Sound passes 1
// and 0.
func (a *Audio) PlayVoice(
	key string,
	wavBytes []byte,
	voices int,
	volScale, pan float64,
) {
	pcm := a.decode(key, wavBytes)
	if pcm == nil {
		return
	}
	vp := a.voices[key]
	if vp == nil {
		vp = &voicePool{voices: make([]voice, max(voices, 1))}
		a.voices[key] = vp
	}
	i := vp.pick(func(i int) bool {
		p := vp.voices[i].p
		return p != nil && p.IsPlaying()
	})
	if i < 0 {
		return
	}
	if old := vp.voices[i].p; old != nil {
		old.Pause()
	}
	if pan != 0 {
		pcm = panPCM(pcm, pan) // a copy: the cached PCM stays unpanned
	}
	p := a.ctx.NewPlayerF32FromBytes(pcm)
	p.SetVolume(a.volSound * volScale)
	vp.voices[i] = voice{p: p, scale: volScale}
	p.Play()
}

// StopEffects silences the effect channels and the sound variables' voices,
// leaving the music channel alone: sounds belong to the scene that started
// them.
func (a *Audio) StopEffects() {
	for ch, p := range a.chans {
		if ch == musicChannel || p == nil {
			continue
		}
		p.Pause()
		delete(a.chans, ch)
	}
	for key, vp := range a.voices {
		for _, vc := range vp.voices {
			if vc.p != nil {
				vc.p.Pause()
			}
		}
		delete(a.voices, key)
	}
}

// panPCM copies pcm with one side attenuated, mirroring the engine's DirectSound
// pan: the near side keeps full level and the far side fades. The cache format is
// interleaved stereo float32, so a frame is 8 bytes (L, R).
func panPCM(pcm []byte, pan float64) []byte {
	out := make([]byte, len(pcm))
	copy(out, pcm)
	k := float32(1 - math.Abs(pan))
	off := 0 // pan > 0 (right) fades the left sample, and vice versa
	if pan < 0 {
		off = 4
	}
	for i := 0; i+8 <= len(out); i += 8 {
		scaleF32(out[i+off:i+off+4], k)
	}
	return out
}

// scaleF32 multiplies one little-endian float32 sample in place.
func scaleF32(b []byte, k float32) {
	v := math.Float32frombits(binary.LittleEndian.Uint32(b))
	binary.LittleEndian.PutUint32(b, math.Float32bits(v*k))
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
