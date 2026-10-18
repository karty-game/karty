"""Whole-body skeletal locomotion; no image-space limb reconstruction."""
import math
import bpy
from mathutils import Matrix, Vector

STANCE = 0.44
STANCE_TRAVEL = 0.70
MODEL_SCALE = 1.12
STRIDE = STANCE_TRAVEL / STANCE * MODEL_SCALE


def smoothstep(value):
    value = max(0, min(1, value))
    return value * value * (3 - 2 * value)


def rotated(point, yaw):
    return Vector((point.x * math.cos(yaw) - point.y * math.sin(yaw),
                   point.x * math.sin(yaw) + point.y * math.cos(yaw), point.z))


def orient(bone, armature, start, end):
    before = armature.matrix_world * bone.matrix
    direction = armature.matrix_world * bone.tail - armature.matrix_world * bone.head
    rotation = direction.rotation_difference(end - start) * before.to_quaternion()
    matrix = rotation.to_matrix().to_4x4()
    matrix.translation = start
    bone.matrix = armature.matrix_world.inverted() * matrix
    # Parent matrices must be evaluated before solving each child.
    bpy.context.scene.update()


def two_bone(armature, upper, lower, target, pole):
    start = armature.matrix_world * upper.head
    delta = target - start
    distance = min(delta.length, (upper.length + lower.length) * 0.985)
    axis = delta.normalized()
    target = start + axis * distance
    along = (upper.length ** 2 - lower.length ** 2 + distance ** 2) / (2 * distance)
    bend = (pole - axis * pole.dot(axis)).normalized()
    joint = start + axis * along + bend * math.sqrt(max(0, upper.length ** 2 - along ** 2))
    orient(upper, armature, start, joint)
    orient(lower, armature, joint, target)
    return target


def torso(armature, phase, idle, yaw):
    # Explicit forward intent replaces the old flattened source back. Shoulder
    # and pelvis rotations oppose one another; the head remains comparatively
    # quiet. Neither the pelvis nor the two planted boots are bitmap offsets.
    scene = bpy.context.scene
    back = armature.pose.bones['Back']
    head = armature.matrix_world * back.head
    tail = armature.matrix_world * back.tail
    lean = math.radians(4 if idle else 8)
    lateral = 0 if idle else math.radians(1.6) * math.sin(2 * math.pi * phase)
    up = rotated(Vector((math.sin(lateral), -math.sin(lean), math.cos(lean))), yaw)
    orient(back, armature, head, head - up * (head - tail).length)
    if not idle:
        shoulder = math.radians(6) * math.cos(2 * math.pi * phase)
        matrix = armature.matrix_world * back.matrix
        rotation = Matrix.Rotation(shoulder, 4, 'Z') * matrix
        rotation.translation = matrix.translation
        back.matrix = armature.matrix_world.inverted() * rotation
        scene.update()
        hip = armature.pose.bones['Hip']
        matrix = armature.matrix_world * hip.matrix
        rotation = Matrix.Rotation(-1.5 * shoulder, 4, 'Z') * matrix
        rotation.translation = matrix.translation
        hip.matrix = armature.matrix_world.inverted() * rotation
        scene.update()


def pelvis_height(phase, idle):
    # Contact absorbs weight, then the pelvis rises over the supporting foot.
    # Idle shares the moving mean for a consistent relaxed stance. The final
    # mesh-floor normalization is measured separately after this solve.
    return 0.925 if idle else 0.925 - 0.022 * math.cos(4 * math.pi * phase)


def limbs(armature, phase, idle, yaw):
    scene = bpy.context.scene
    for leg, side in enumerate(['Left', 'Right']):
        thigh = armature.pose.bones['Thigh.' + side]
        shin = armature.pose.bones['Shin.' + side]
        foot = armature.pose.bones['Foot.' + side]
        t = (phase + leg * 0.5) % 1
        if t < STANCE:
            progress = t / STANCE
            y = -STANCE_TRAVEL / 2 + STANCE_TRAVEL * progress
            lift = 0
            pitch = math.radians(-8 * (1 - smoothstep(progress / 0.20))
                                 + 19 * smoothstep((progress - 0.72) / 0.28))
        else:
            progress = (t - STANCE) / (1 - STANCE)
            # Fast release, tucked passing knee, soft preparation for contact.
            smooth = progress * progress * (3 - 2 * progress)
            y = STANCE_TRAVEL / 2 - STANCE_TRAVEL * smooth
            lift = 0.13 * math.sin(math.pi * progress) ** 1.35
            if progress < 0.45:
                pitch = math.radians(19 - 31 * smoothstep(progress / 0.45))
            else:
                pitch = math.radians(-12 + 4 * smoothstep((progress - 0.45) / 0.55))
        if idle:
            y, lift, pitch = (-0.045 if leg == 0 else 0.045), 0, 0
        target = rotated(Vector((0.095 if leg == 0 else -0.095, y, 0.105 + lift)), yaw)
        target = two_bone(armature, thigh, shin, target,
                          rotated(Vector((0, -1, 0)), yaw))
        rest = armature.matrix_world * foot.bone.matrix_local
        rest_direction = rest.to_3x3() * Vector((0, 1, 0))
        forward = rotated(Vector((0, -math.cos(pitch), -math.sin(pitch))), yaw)
        rotation = rest_direction.rotation_difference(forward) * rest.to_quaternion()
        matrix = rotation.to_matrix().to_4x4()
        matrix.translation = target
        foot.matrix = armature.matrix_world.inverted() * matrix
        scene.update()

    # Flexed elbows carry the equipment near the body. Opposite arm and leg
    # advance together; the buckler has less travel than the sword hand.
    for index, side in enumerate(['Left', 'Right']):
        upper = armature.pose.bones['Upperarm.' + side]
        lower = armature.pose.bones['Forearm.' + side]
        shoulder = rotated(armature.matrix_world * upper.head, -yaw)
        sign = 1 if index == 0 else -1
        swing = 0 if idle else math.cos(2 * math.pi * phase)
        amplitude = 0.085 if index == 0 else 0.19
        drop = 0.445 if index == 0 else (0.43 if idle else 0.37)
        target = Vector((shoulder.x + sign * 0.105,
                         shoulder.y - (0.065 if index == 0 else 0.105)
                         + sign * amplitude * swing,
                         shoulder.z - drop
                         + (0 if idle else 0.018 * math.sin(4 * math.pi * phase))))
        two_bone(armature, upper, lower, rotated(target, yaw),
                 rotated(Vector((sign * 0.55, 0.85, -0.15)), yaw))
