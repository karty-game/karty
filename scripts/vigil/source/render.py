"""Bake Vigil's original-model-derived, coherent whole-body walk.

Run with Blender 2.79b, opening pilgrim.blend before this script. Each imported
model part has different rest-bone axes; skin deformation transfers in world
space, evaluating every parent before its children. No image-space body parts
or limb stretching are used. See README.md for license and regeneration.
"""
import bpy,math,os,sys,argparse,json
if bpy.app.version[:2] != (2,79):
 raise RuntimeError("Vigil character baking requires Blender 2.79b (legacy Blender Internal renderer)")
parser=argparse.ArgumentParser()
parser.add_argument("--output",required=True)
parser.add_argument("--idle-only",action="store_true")
parser.add_argument("--preview",action="store_true")
args=parser.parse_args(sys.argv[sys.argv.index("--")+1:])
os.makedirs(args.output,exist_ok=True)
from mathutils import Quaternion,Vector,Matrix
s=bpy.context.scene;platformScales={o.name:o.scale.copy() for o in s.objects if o.name.startswith('RenderPlatform')};arm=bpy.data.objects['MaleArm'];arm.animation_data.action=bpy.data.actions['Action']
sys.path.insert(0,os.path.dirname(os.path.abspath(__file__)))
sys.dont_write_bytecode=True
import costume,locomotion
costume.apply(arm)
rigs=[o for o in s.objects if o.type=='ARMATURE' and o!=arm]
for o in rigs:
 if o.animation_data:o.animation_data_clear()
samples=[]
for f in range(5,13):
 s.frame_set(f);s.update();samples.append({b.name:(b.rotation_quaternion.copy(),b.location.copy()) for b in arm.pose.bones})
averages={}
for name in arm.pose.bones.keys():
 qs=[x[name][0] for x in samples];q=qs[0].copy()
 for i,v in enumerate(qs[1:]):q=q.slerp(v,1./(i+2))
 averages[name]=(q,sum((x[name][1] for x in samples),Vector())/8)
arm.animation_data_clear()
diagnostics=[]
for row in ([0,1,2] if args.preview else range(8)):
 for i in ([24] if args.idle_only else ([0,3,6,9,12,15,18,21,24] if args.preview else range(25))):
  idle=i==24
  phase=0 if idle else i/24.;index=phase*8;k=int(index);blend=index-k
  for b in arm.pose.bones:
   q0,l0=samples[k][b.name];q1,l1=samples[(k+1)%8][b.name];q=q0.slerp(q1,blend);loc=l0.lerp(l1,blend);aq,al=averages[b.name]
   weight=.35 if b.name=='Head' else .85
   q=aq.slerp(q,weight);loc=al+(loc-al)*.6
   if idle:q=aq;loc=al
   b.rotation_quaternion=q;b.location=loc
  for o in s.objects:
   if o.name.startswith('RenderPlatform'):o.rotation_euler.z=-row*math.pi/4;o.location.z=0;o.scale=platformScales[o.name]
  s.update()
  yaw=-row*math.pi/4
  locomotion.torso(arm,phase,idle,yaw)
  hips=[arm.matrix_world*arm.pose.bones['Thigh.'+side].head for side in ['Left','Right']]
  correction=locomotion.pelvis_height(phase,idle)-sum(p.z for p in hips)/2
  for o in s.objects:
   if o.name.startswith('RenderPlatform'):o.location.z=correction
  s.update()
  locomotion.limbs(arm,phase,idle,yaw)
  for o in rigs:
   inv=o.matrix_world.inverted()
   for b in o.pose.bones:
    if b.name in arm.pose.bones:
     pb=arm.pose.bones[b.name];delta=arm.matrix_world*pb.matrix*(arm.matrix_world*pb.bone.matrix_local).inverted()
     b.matrix=inv*delta*o.matrix_world*b.bone.matrix_local
     s.update()
   s.update()
  mesh=bpy.data.objects['HumanMale'];temp=mesh.to_mesh(s,True,'RENDER');points=[mesh.matrix_world*v.co for v in temp.vertices];floor=min(v.z for v in points);bpy.data.meshes.remove(temp)
  for o in s.objects:
   if o.name.startswith('RenderPlatform'):o.location.z+=-floor
  s.update()
  for o in s.objects:
   if o.name.startswith('RenderPlatform'):o.scale=platformScales[o.name]*1.12;o.location.z*=1.12
  s.update()
  diagnostics.append({'direction':row,'frame':i,'floorCorrection':floor,'bones':{name:{'head':list(arm.matrix_world*arm.pose.bones[name].head),'tail':list(arm.matrix_world*arm.pose.bones[name].tail)} for name in ['Back','Hip','Head','Upperarm.Left','Forearm.Left','Upperarm.Right','Forearm.Right','Thigh.Left','Shin.Left','Foot.Left','Thigh.Right','Shin.Right','Foot.Right']}})
  print('VIGIL_POSE',row,i,'floor',floor)
  name='idle-%d.png'%row if idle else 'walk-%d-%02d.png'%(row,i)
  s.render.filepath=os.path.join(args.output,name);bpy.ops.render.render(write_still=True)

with open(os.path.join(args.output,'pilgrim-render.json'),'w') as file:
 json.dump({'canvasSize':192,'rootX':96,'rootY':160,'projectedSize':2.05*192/157,'frames':24,'directions':8,'stride':locomotion.STRIDE},file,indent=2)
 file.write('\n')

with open(os.path.join(args.output,'pose-diagnostics.json'),'w') as file:
 json.dump(diagnostics,file,indent=2)
 file.write('\n')
