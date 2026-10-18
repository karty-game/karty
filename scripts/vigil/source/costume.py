"""Render-time costume finishing for the retained, attributed Pilgrim model.

All detail is modeled or shaded before whole-body rendering. The skeletal gait,
equipment attachment solve, camera projection and sprite roots stay unchanged.
"""
import math
import bpy


def cloth(name, color, variation=0.17):
    material = bpy.data.materials[name]
    material.diffuse_color = color
    material.diffuse_intensity = 0.8
    material.specular_intensity = 0.035
    # Object-space variation survives all eight views without crawling between
    # frames. Broad, low-contrast mottling gives cloth weight at game size.
    texture = bpy.data.textures.new("Vigil " + name + " weave", type='CLOUDS')
    texture.noise_scale = 0.065
    texture.noise_depth = 1
    texture.use_color_ramp = True
    texture.color_ramp.elements[0].color = (0.48, 0.42, 0.34, 1)
    texture.color_ramp.elements[1].color = (1, 0.96, 0.87, 1)
    slot = material.texture_slots.add()
    slot.texture = texture
    slot.texture_coords = 'ORCO'
    slot.blend_type = 'MULTIPLY'
    slot.diffuse_color_factor = variation
    return material


def attached_mesh(name, vertices, faces, armature, material):
    mesh = bpy.data.meshes.new(name)
    mesh.from_pydata(vertices, [], faces)
    mesh.update()
    obj = bpy.data.objects.new(name, mesh)
    bpy.context.scene.objects.link(obj)
    obj.parent = armature.parent
    obj.data.materials.append(material)
    group = obj.vertex_groups.new(name='Back')
    group.add(list(range(len(vertices))), 1.0, 'REPLACE')
    modifier = obj.modifiers.new(name='Pilgrim costume skin', type='ARMATURE')
    modifier.object = armature
    # Vertices are authored in world rest space. The modifier evaluates that
    # same rest pose before applying the complete torso's current deformation.
    for polygon in mesh.polygons:
        polygon.use_smooth = True
    return obj


def shoulder_mantle(armature, material):
    segments = 32
    vertices = []
    for ring in range(3):
        for i in range(segments):
            angle = 2 * math.pi * i / segments
            if ring == 0:
                x, y, z = 0.12, 0.11, 1.565
            elif ring == 1:
                x, y, z = 0.278, 0.18, 1.485
            else:
                x, y = 0.275, 0.19
                # The shorter front reveals the tunic and belt; the back has
                # a little more cloth, like a worn traveling shoulder cape.
                drop = 0.065 if math.sin(angle) < 0 else 0.115
                z = 1.43 - drop * abs(math.sin(angle))
            # Slight alternating folds catch the key light, without serrating
            # the silhouette or adding independent moving bitmap pieces.
            fold = 1 + (0.025 * math.cos(angle * 8) if ring else 0)
            vertices.append((x * math.cos(angle) * fold,
                             y * math.sin(angle) * fold + 0.025, z))
    faces = []
    for ring in range(2):
        for i in range(segments):
            a = ring * segments + i
            b = ring * segments + (i + 1) % segments
            faces.append((a, b, b + segments, a + segments))
    attached_mesh('Vigil shoulder mantle', vertices, faces, armature, material)


def apply(armature):
    # The retained source buckler was oversized for a light traveling figure.
    # Its mesh origin is the handle center, so this keeps the arm attachment.
    bpy.data.objects['LightShield'].scale *= 0.85
    cloth('WhiteCloth', (0.37, 0.32, 0.23))
    cloth('TanCloth', (0.32, 0.27, 0.19))
    cloth('BlueCloth', (0.085, 0.064, 0.042), 0.12)
    hood = cloth('Leather.001', (0.205, 0.037, 0.029), 0.22)
    mantle = hood.copy()
    mantle.name = 'Vigil worn oxblood mantle'
    mantle.diffuse_color = (0.16, 0.043, 0.03)
    shoulder_mantle(armature, mantle)
    for name, color in [
        ('Leather', (0.16, 0.11, 0.064)),
        ('LightLeather', (0.17, 0.102, 0.051)),
        ('Black', (0.063, 0.047, 0.031)),
        ('Wood.004', (0.14, 0.095, 0.052)),
        ('WoodDark', (0.105, 0.075, 0.046)),
        ('RoughSteel.002', (0.16, 0.17, 0.16)),
        ('Silver', (0.36, 0.38, 0.37)),
        ('Metal', (0.29, 0.25, 0.16)),
    ]:
        material = bpy.data.materials[name]
        material.diffuse_color = color
        material.specular_intensity = 0.14 if name in (
            'Silver', 'RoughSteel.002', 'Metal') else 0.025
        material.specular_hardness = 24
