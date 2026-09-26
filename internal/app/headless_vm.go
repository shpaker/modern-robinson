//go:build ebitenginevm

package app

// headless is a run under the Ebitengine VM host (.vmdriver): scripted clicks
// and snapshots of the bare frame. The tube starts off (tubeOn) and the
// window's keys write no settings (keepSwitch).
const headless = true
