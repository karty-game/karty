"""Compose Last Carriage, an original 32-bar minor-key lounge foxtrot.

Python standard library only. Rendering needs FluidSynth 2.2.5's shared library
and the pinned GeneralUser GS bank; see assets/audio/README.md. Nothing is
downloaded implicitly. The checked-in WAV is all the game/build pipeline needs.
"""

import argparse
from array import array
import ctypes as C
import ctypes.util
import hashlib
import json
import math
from pathlib import Path
import random
import struct
import wave

RATE = 48000
BPM = 92
BEATS = 128
FRAMES = round(BEATS * 60 * RATE / BPM)
BANK_SHA = "9575028c7a1f589f5770fccc8cff2734566af40cd26ed836944e9a5152688cfe"
OUTPUT = Path("samples/last-train/assets/audio")


def compose():
    rng = random.Random(1933)
    events = []

    def note(channel, pitch, at, length, velocity):
        # Small reproducible differences in touch, articulation and onset.
        at = max(0, at + rng.uniform(-0.009, 0.009))
        velocity = max(1, min(127, velocity + rng.randint(-3, 3)))
        events.append((at, 1, channel, pitch, velocity))
        events.append((min(BEATS - 0.008, at + length), 0, channel, pitch, 0))

    # Root, fifth, close piano voicing. Descending bass gives A its identity.
    a = [
        (45, 52, [57, 60, 64, 66]),  # Am6
        (44, 52, [56, 60, 64, 69]),  # Am(maj7)/G#
        (43, 50, [57, 60, 64, 67]),  # Am7/G
        (42, 49, [57, 60, 64, 66]),  # D9/F#
        (38, 45, [57, 60, 64, 65]),  # Dm9
        (40, 47, [57, 60, 64, 66]),  # Am6/E
        (35, 42, [57, 59, 62, 65]),  # Bm7b5
        (40, 47, [56, 59, 62, 65]),  # E7b9 turnaround
    ]
    bridge = [
        (36, 43, [55, 60, 64, 69]),  # C6
        (37, 43, [55, 58, 61, 64]),  # C#dim7
        (38, 45, [57, 60, 64, 65]),  # Dm9
        (43, 50, [53, 57, 59, 64]),  # G13
        (40, 47, [55, 59, 62, 67]),  # Em7
        (45, 52, [55, 58, 61, 64]),  # A7b9
        (38, 45, [53, 57, 60, 64]),  # Dm9
        (40, 47, [56, 59, 62, 65]),  # E7b9
    ]
    harmony = a + a + bridge + a
    # Each tuple is (onset in beats, pitch, held beats). Rests are intentional.
    theme = [
        [(0.12, 76, 1.25), (1.62, 72, .34), (2, 69, .85), (3.6, 71, .28)],
        [(0, 72, 1.65), (2, 76, .82), (3, 75, .32), (3.6, 76, .28)],
        [(0, 79, 1.1), (1.6, 76, .32), (2, 72, 1.45)],
        [(0.15, 74, 1.2), (1.6, 72, .32), (2, 69, 1.3)],
        [(0, 77, .85), (1, 76, .36), (1.6, 74, .32), (2, 72, 1.4)],
        [(0, 76, 1.2), (1.6, 72, .32), (2, 69, .82), (3.6, 66, .27)],
        [(0, 65, .82), (1, 69, .85), (2, 71, 1.2), (3.6, 74, .28)],
        [(0, 72, .85), (1, 71, .35), (1.6, 68, .32), (2, 64, 1.3)],
    ]
    middle = [
        [(0.1, 76, .85), (1, 79, .85), (2, 81, 1.5)],
        [(0, 79, 1.25), (1.6, 76, .3), (2, 73, 1.25)],
        [(0, 77, 1.2), (1.6, 76, .3), (2, 74, .85), (3.6, 72, .28)],
        [(0, 71, 1.1), (1.6, 69, .3), (2, 67, 1.4)],
        [(0.1, 71, .85), (1, 74, .85), (2, 79, 1.35)],
        [(0, 76, 1.2), (1.6, 73, .3), (2, 70, .85), (3.6, 69, .28)],
        [(0, 69, 1.2), (1.6, 72, .3), (2, 77, 1.35)],
        [(0, 76, .85), (1, 74, .35), (1.6, 71, .3), (2, 68, 1.3)],
    ]
    for bar, (root, fifth, chord) in enumerate(harmony):
        start = bar * 4
        section = bar // 8
        # Two-beat bass in A; a walking bridge lifts the middle section.
        bass = [(0, root), (2, fifth)]
        if section == 2:
            next_root = harmony[(bar + 1) % 32][0]
            bass = [(0, root), (1, root + 7), (2, fifth), (3, next_root - 1)]
        for offset, pitch in bass:
            note(2, pitch, start + offset, .72 if section == 2 else 1.3, 68)

        # Soft piano shells, loosely rolled, with space for the melody.
        for offset, velocity in [(0.08, 43), (1.62, 38), (3.06, 40)]:
            for index, pitch in enumerate(chord):
                note(0, pitch, start + offset + index * .014, .62, velocity)
        # Quiet acoustic guitar on the backbeats, not an electronic pad.
        for offset in [1, 3]:
            for index, pitch in enumerate(chord[:3]):
                note(3, pitch + 12, start + offset + index * .012, .2, 37)

        lead = 0 if section in (0, 3) else 1
        phrase = (middle if section == 2 else theme)[bar % 8]
        for offset, pitch, length in phrase:
            note(lead, pitch, start + offset, length, 62 if lead == 0 else 57)
        # Clarinet answers the final piano statement in its lower register.
        if section == 3 and bar % 8 in (1, 3, 5):
            note(1, chord[-1], start + 3.05, .65, 46)
        # Brush kit: a soft sweep and backbeat tap, no synthesized noise bed.
        for offset in [0, 2]:
            note(9, 39, start + offset, .45, 30)
        for offset in [1, 3]:
            note(9, 38, start + offset, .25, 38)
        if section == 2:
            for offset in [.6, 1.6, 2.6, 3.6]:
                note(9, 42, start + offset, .12, 23)
    return sorted(events)


def midi(events):
    def vlq(number):
        result = [number & 127]
        while number >> 7:
            number >>= 7
            result.insert(0, 128 | (number & 127))
        return bytes(result)

    track = bytearray(b'\x00\xff\x51\x03' + round(60_000_000 / BPM).to_bytes(3, 'big'))
    track.extend(b'\x00\xff\x58\x04\x04\x02\x18\x08')
    for channel, program, pan, volume in [(0, 0, 53, 95), (1, 71, 73, 86), (2, 32, 60, 106), (3, 24, 75, 66), (9, 40, 64, 68)]:
        track.extend(bytes([0, 0xC0 | channel, program, 0, 0xB0 | channel, 10, pan, 0, 0xB0 | channel, 7, volume]))
    previous = 0
    for at, on, channel, pitch, velocity in events:
        tick = round(at * 480)
        track.extend(vlq(tick - previous))
        track.extend(bytes([(0x90 if on else 0x80) | channel, pitch, velocity]))
        previous = tick
    track.extend(vlq(BEATS * 480 - previous) + b'\xff\x2f\x00')
    return b'MThd' + struct.pack('>IHHH', 6, 0, 1, 480) + b'MTrk' + struct.pack('>I', len(track)) + track


def render(events, bank, library):
    fluid = C.CDLL(library)

    def bind(name, result, *args):
        function = getattr(fluid, name)
        function.restype, function.argtypes = result, args
        return function

    ptr, integer, char = C.c_void_p, C.c_int, C.c_char_p
    settings = bind('new_fluid_settings', ptr)()
    number = bind('fluid_settings_setnum', integer, ptr, char, C.c_double)
    setting_int = bind('fluid_settings_setint', integer, ptr, char, integer)
    number(settings, b'synth.sample-rate', RATE)
    number(settings, b'synth.gain', .5)
    number(settings, b'synth.reverb.room-size', .35)
    number(settings, b'synth.reverb.damp', .6)
    number(settings, b'synth.reverb.width', 35)
    number(settings, b'synth.reverb.level', .16)
    setting_int(settings, b'synth.chorus.active', 0)
    setting_int(settings, b'synth.cpu-cores', 1)
    synth = bind('new_fluid_synth', ptr, ptr)(settings)
    try:
        sf = bind('fluid_synth_sfload', integer, ptr, char, integer)(synth, str(bank).encode(), 1)
        if sf < 0:
            raise RuntimeError('SoundFont failed to load')
        select = bind('fluid_synth_program_select', integer, ptr, integer, integer, integer, integer)
        cc = bind('fluid_synth_cc', integer, ptr, integer, integer, integer)
        for channel, program, pan, volume in [(0, 0, 53, 95), (1, 71, 73, 86), (2, 32, 60, 106), (3, 24, 75, 66), (9, 40, 64, 68)]:
            if select(synth, channel, sf, 128 if channel == 9 else 0, program) != 0:
                raise RuntimeError(f'missing instrument {channel}:{program}')
            cc(synth, channel, 10, pan)
            cc(synth, channel, 7, volume)
        on = bind('fluid_synth_noteon', integer, ptr, integer, integer, integer)
        off = bind('fluid_synth_noteoff', integer, ptr, integer, integer)
        write = bind('fluid_synth_write_float', integer, ptr, integer, ptr, integer, integer, ptr, integer, integer)
        buffer = (C.c_float * 4096)()
        samples = array('f')
        # Two warm-up cycles establish releases/reverb across the loop boundary.
        for cycle in range(3):
            cursor = 0

            def advance(target):
                nonlocal cursor
                while cursor < target:
                    count = min(2048, target - cursor)
                    if write(synth, count, buffer, 0, 2, buffer, 1, 2) != 0:
                        raise RuntimeError('render failed')
                    if cycle == 2:
                        samples.frombytes(C.string_at(buffer, count * 8))
                    cursor += count

            for at, sounding, channel, pitch, velocity in events:
                advance(round(at * FRAMES / BEATS))
                if sounding:
                    on(synth, channel, pitch, velocity)
                else:
                    off(synth, channel, pitch)
            advance(FRAMES)
        return samples
    finally:
        bind('delete_fluid_synth', None, ptr)(synth)
        bind('delete_fluid_settings', None, ptr)(settings)


def master(samples):
    # Linear gain only: no tanh distortion, vinyl noise, bit reduction or hiss.
    means = [sum(samples[channel::2]) / FRAMES for channel in range(2)]
    peak = max(abs(value - means[index % 2]) for index, value in enumerate(samples))
    gain = .28 / peak
    pcm = array('h', (round((value - means[index % 2]) * gain * 32767) for index, value in enumerate(samples)))
    # 2 ms correction of the residual boundary mismatch, not a silence/fade dip.
    # This also absorbs the tiny non-periodic modulation differences in the bank.
    for channel in range(2):
        delta = pcm[channel] - pcm[-2 + channel]
        for frame in range(96):
            weight = .5 - .5 * math.cos(math.pi * frame / 95)
            index = (FRAMES - 96 + frame) * 2 + channel
            pcm[index] += round(delta * weight)
    return pcm


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--soundfont', type=Path, required=True)
    parser.add_argument('--library', default=ctypes.util.find_library('fluidsynth') or 'libfluidsynth.so.3')
    args = parser.parse_args()
    if hashlib.sha256(args.soundfont.read_bytes()).hexdigest() != BANK_SHA:
        raise SystemExit('SoundFont checksum differs from the credited bank')
    events = compose()
    OUTPUT.mkdir(parents=True, exist_ok=True)
    (OUTPUT / 'last-carriage.mid').write_bytes(midi(events))
    pcm = master(render(events, args.soundfont, args.library))
    if struct.pack('=H', 1) != struct.pack('<H', 1):
        pcm.byteswap()
    path = OUTPUT / 'last-carriage.wav'
    with wave.open(str(path), 'wb') as output:
        output.setparams((2, 2, RATE, FRAMES, 'NONE', 'not compressed'))
        output.writeframes(pcm.tobytes())
    rms = math.sqrt(sum(value * value for value in pcm) / len(pcm)) / 32768
    print(json.dumps({'track': str(path), 'seconds': FRAMES / RATE, 'bars': 32, 'bpm': BPM,
                      'peak': max(abs(value) for value in pcm) / 32768, 'rms': rms,
                      'boundaryStep': [abs(pcm[c] - pcm[-2+c]) for c in range(2)],
                      'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}, indent=2))


if __name__ == '__main__':
    main()
