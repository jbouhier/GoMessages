# Synthesized music bed: warm pad, plucked arpeggio, soft kick and hats. 120 bpm.
import numpy as np, wave

SR, DUR, BPM = 44100, 38.5, 120
N = int(SR * DUR)
t = np.arange(N) / SR
beat = 60 / BPM
out = np.zeros(N)

def midi(m): return 440 * 2 ** ((m - 69) / 12)
def env_adsr(n, a, r):
    e = np.ones(n); na, nr = int(a * SR), int(r * SR)
    e[:na] = np.linspace(0, 1, na); e[-nr:] *= np.linspace(1, 0, nr); return e
def lowpass(x, fc):
    a = np.exp(-2 * np.pi * fc / SR); y = np.zeros_like(x); acc = 0.0
    for i in range(len(x)):
        acc = (1 - a) * x[i] + a * acc; y[i] = acc
    return y

# Fmaj9 - Am7 - Dm9 - Bbmaj7 (2 bars each)
chords = [[53, 60, 64, 67, 69], [57, 60, 64, 67, 71], [50, 57, 60, 64, 65], [46, 53, 57, 62, 65]]
bar = 4 * beat
pad = np.zeros(N)
for ci in range(int(DUR / (2 * bar)) + 1):
    ch = chords[ci % 4]; s = int(ci * 2 * bar * SR); n = min(int(2 * bar * SR) + int(0.6 * SR), N - s)
    if n <= 0: break
    tt = np.arange(n) / SR; seg = np.zeros(n)
    for m in ch:
        f = midi(m)
        for det in (-0.12, 0.12):
            ph = 2 * np.pi * f * (1 + det / 100) * tt
            seg += (np.sin(ph) + 0.35 * np.sin(2 * ph) + 0.15 * np.sin(3 * ph))
    pad[s:s + n] += seg * env_adsr(n, 0.5, 0.8)
pad = lowpass(pad, 1400) * 0.022

# Pluck arpeggio on 8ths from 2.8s to 30s and on the end card.
pl = np.zeros(N)
pattern = [0, 2, 3, 4, 3, 2, 1, 2]
step = beat / 2
for k in range(int(DUR / step)):
    st = k * step
    if not (2.8 <= st < 29.9 or 33.6 <= st < 37.0): continue
    ch = chords[int(st / (2 * bar)) % 4]
    m = ch[pattern[k % 8]] + 12
    n = int(0.6 * SR); s = int(st * SR); n = min(n, N - s)
    tt = np.arange(n) / SR; f = midi(m)
    tone = np.sin(2 * np.pi * f * tt) + 0.3 * np.sin(4 * np.pi * f * tt) * np.exp(-tt * 18)
    pl[s:s + n] += tone * np.exp(-tt * 7) * (0.9 if k % 2 == 0 else 0.6)
pl = lowpass(pl, 3500) * 0.05

# Kick on beats, hats on off-beats, in the feature section.
dr = np.zeros(N)
rng = np.random.default_rng(3)
for k in range(int(DUR / beat)):
    st = k * beat
    if 4.0 <= st < 29.9:
        n = int(0.35 * SR); s = int(st * SR); n = min(n, N - s); tt = np.arange(n) / SR
        f = 45 + 75 * np.exp(-tt * 30)
        dr[s:s + n] += np.sin(2 * np.pi * np.cumsum(f) / SR) * np.exp(-tt * 9) * 0.55
        hs = int((st + beat / 2) * SR); hn = min(int(0.06 * SR), N - hs)
        if hn > 0:
            nz = rng.standard_normal(hn); nz = nz - lowpass(nz, 6000)
            dr[hs:hs + hn] += nz * np.exp(-np.arange(hn) / SR * 60) * 0.06
dr *= 0.5

# Soft risers into the window entrance and the end card.
fx = np.zeros(N)
for (a, b) in [(1.9, 2.8), (32.8, 33.6)]:
    s, e = int(a * SR), int(b * SR); n = e - s
    nz = rng.standard_normal(n); nz = lowpass(nz, 2500) - lowpass(nz, 300)
    fx[s:e] += nz * np.linspace(0, 1, n) ** 2 * 0.25

mix = pad + pl + dr + fx
fade = np.ones(N); fade[:int(0.8 * SR)] = np.linspace(0, 1, int(0.8 * SR)); fade[-int(2.0 * SR):] = np.linspace(1, 0, int(2.0 * SR))
mix *= fade
mix = np.tanh(mix * 1.6) / 1.6
mix /= np.max(np.abs(mix)) / 0.8
st = np.stack([mix, np.roll(mix, int(0.012 * SR)) * 0.96 + mix * 0.04], axis=1)
with wave.open(".work/music.wav", "wb") as w:
    w.setnchannels(2); w.setsampwidth(2); w.setframerate(SR)
    w.writeframes((st * 32767).astype(np.int16).tobytes())
print("ok")
