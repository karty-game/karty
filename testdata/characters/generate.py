#!/usr/bin/env python3
"""Regenerate the artwork-free skinned triangle and named animation fixtures."""

import json
from pathlib import Path
import struct


def glb(name, nodes, geometry=False):
    blob = bytearray()
    views, accessors = [], []

    def values(data, kind, dimensions, integer=False):
        start = len(blob)
        blob.extend(struct.pack("<" + ("H" if integer else "f") * len(data), *data))
        views.append({"buffer": 0, "byteOffset": start, "byteLength": len(blob) - start})
        accessors.append({"bufferView": len(views) - 1, "componentType": 5123 if integer else 5126,
                          "count": len(data) // dimensions, "type": kind})
        return len(accessors) - 1

    document = {"asset": {"version": "2.0"}, "nodes": nodes}
    if geometry:
        attributes = {
            "POSITION": values([0, 0, 0, 1, 0, 0, 0, 1, 0], "VEC3", 3),
            "NORMAL": values([0, 0, 1] * 3, "VEC3", 3),
            "JOINTS_0": values([0] * 12, "VEC4", 4, integer=True),
            "WEIGHTS_0": values([1, 0, 0, 0] * 3, "VEC4", 4),
        }
        document["meshes"] = [{"primitives": [{"attributes": attributes}]}]
        document["skins"] = [{"joints": [1]}]
    else:
        times = values([0, 1], "SCALAR", 1)
        samples = values([0, 0, 0, 1, 0, 0], "VEC3", 3)
        document["animations"] = [{"name": "Walk", "samplers": [
            {"input": times, "output": samples, "interpolation": "LINEAR"}],
            "channels": [{"sampler": 0, "target": {"node": 0, "path": "translation"}}]}]

    document.update(buffers=[{"byteLength": len(blob)}], bufferViews=views, accessors=accessors)
    encoded = json.dumps(document, separators=(",", ":")).encode()
    encoded += b" " * (-len(encoded) % 4)
    blob += b"\0" * (-len(blob) % 4)
    output = struct.pack("<4sII", b"glTF", 2, 28 + len(encoded) + len(blob))
    output += struct.pack("<I4s", len(encoded), b"JSON") + encoded
    output += struct.pack("<I4s", len(blob), b"BIN\0") + blob
    Path(__file__).with_name(name).write_bytes(output)


glb("model.glb", [{"mesh": 0, "skin": 0}, {"name": "hip"}], geometry=True)
glb("animations.glb", [{"name": "hip"}])
