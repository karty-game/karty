// Standalone moving art review. Gameplay qualification remains check-vigil-web.
import { readFile, mkdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const root = resolve(process.env.KARTY_VIGIL_PROJECT || "samples/vigil");
const metadata = JSON.parse(await readFile(resolve(root, "levels/crypt/walk.generated.json"), "utf8"));
const frames = metadata.frames;
const directions = metadata.directions;
const baselineDirectory = process.argv.find((arg) => arg.startsWith("--baseline="))?.slice(11);
const baseline = baselineDirectory
  ? JSON.parse(await readFile(resolve(baselineDirectory, "walk.generated.json"), "utf8"))
  : null;
if (baseline) {
  baseline.moveSpeed ??= 2.8;
  baseline.hastenSpeed ??= 4.5;
}
const png = async (name) =>
  "data:image/png;base64," + (await readFile(resolve(root, "levels/crypt", name + ".png"))).toString("base64");
const rows = [];
for (let row = 0; row < directions; row++) {
  const before = [];
  const after = [];
  for (let pose = 0; pose < (baseline?.frames ?? 0); pose++) {
    const name = baseline.compositeNames[row * baseline.frames + pose];
    before.push(
      "data:image/png;base64," + (await readFile(resolve(baselineDirectory, name + ".png"))).toString("base64"),
    );
  }
  for (let pose = 0; pose < frames; pose++) after.push(await png(metadata.walkNames[row * frames + pose]));
  const beforeIdle = baselineDirectory
    ? "data:image/png;base64," +
      (
        await readFile(resolve(baselineDirectory, baseline.compositeNames[baseline.frames * directions + row] + ".png"))
      ).toString("base64")
    : null;
  rows.push({ before, beforeIdle, after, idle: await png(metadata.idleNames[row]) });
}
const output = resolve("dist/validation/vigil/animation");
await mkdir(output, { recursive: true });
await writeFile(
  resolve(output, "index.html"),
  `<!doctype html>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Vigil walk review</title>
<style>
body{background:#18191b;color:#e7dfd0;font:16px system-ui;max-width:1000px;margin:24px auto;padding:0 16px}
h1{font-size:24px}button,select,input{font:inherit;margin:4px;padding:6px;accent-color:#b89255}
header{position:sticky;top:0;background:#18191bef;padding:8px 0;z-index:1}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(320px,1fr));gap:12px}
article{background:#242528;border:1px solid #48433a;padding:12px;border-radius:6px}
article h2{font-size:16px;margin:0 0 8px}article p{display:flex;justify-content:space-around;font-size:13px;color:#bcb6ac}
canvas{display:block;width:100%;height:auto;background:radial-gradient(ellipse at center,#373536,#252426 75%)}
.note{font-size:14px;color:#c0b9ae;line-height:1.5}#phase{width:220px}
</style>
<h1>Vigil walk review</h1>
<p class="note">Moving extracted artwork at the normal game size and camera zoom.
This is an animation review; the browser gameplay check separately verifies actual WASM playback and movement.</p>
<header><button id="playing">Pause</button><button id="idle">Show idle</button>
<select id="speed"><option value="${metadata.moveSpeed}">Move</option><option value="${metadata.hastenSpeed}">Hasten</option><option value="${metadata.moveSpeed / 4}">Quarter speed</option></select>
<label>Travel <input id="phase" type="range" min="0" max="2" step="0.001" value="0"></label>
<output id="pose"></output></header><div class="grid"></div>
<script>
const data=${JSON.stringify({ ...metadata, rows, baseline, baselineLabel: "Previous preview" })};
const labels=['South','Southwest','West','Northwest','North','Northeast','East','Southeast'];
const previousSize=(data.baseline?.projectedSize??0)*720/13.2,currentSize=data.projectedSize*720/13.2,width=Math.ceil(previousSize+currentSize),baseline=Math.ceil(Math.max(previousSize*(data.baseline?.originY??0),currentSize*data.originY))+4,height=Math.ceil(baseline+Math.max(previousSize*(1-(data.baseline?.originY??0)),currentSize*(1-data.originY))+8);
let running=true,idle=false,travel=0,previousTravel=0,last=0;
const load=src=>new Promise((resolve,reject)=>{const im=new Image();im.onload=()=>resolve(im);im.onerror=reject;im.src=src});
const ready=Promise.all(data.rows.map(async(row,i)=>{
 const article=document.createElement('article');article.innerHTML='<h2>'+labels[i]+'</h2><canvas></canvas><p>'+(data.baseline?'<span>'+data.baselineLabel+'</span>':'')+'<span>Current locomotion</span></p>';document.querySelector('.grid').append(article);
 const canvas=article.querySelector('canvas');canvas.width=width*2;canvas.height=height*2;canvas.style.width=width+'px';article.style.minWidth=width+'px';
 return {canvas,before:await Promise.all(row.before.map(load)),beforeIdle:row.beforeIdle?await load(row.beforeIdle):null,after:await Promise.all(row.after.map(load)),idle:await load(row.idle)};
}));
document.querySelector('#playing').onclick=e=>{running=!running;e.target.textContent=running?'Pause':'Play'};
document.querySelector('#idle').onclick=e=>{idle=!idle;e.target.textContent=idle?'Show walk':'Show idle'};
document.querySelector('#phase').max=data.stride;
document.querySelector('#phase').oninput=e=>{travel=+e.target.value;previousTravel=travel/data.stride*(data.baseline?.stride??0);running=false;document.querySelector('#playing').textContent='Play'};
ready.then(rows=>requestAnimationFrame(function draw(now){
 if(last&&running&&!idle){const dt=(now-last)/1000,selection=document.querySelector('#speed');travel+=dt*+selection.value;if(data.baseline)previousTravel+=dt*(selection.selectedIndex===1?data.baseline.hastenSpeed:selection.selectedIndex===2?data.baseline.moveSpeed/4:data.baseline.moveSpeed);}last=now;
 const phase=(travel%data.stride)/data.stride,frame=Math.floor(phase*data.frames)%data.frames;
 document.querySelector('#phase').value=travel%data.stride;document.querySelector('#pose').textContent=idle?'Idle':'Pose '+(frame+1)+' / '+data.frames;
 for(const row of rows){const c=row.canvas.getContext('2d');c.clearRect(0,0,row.canvas.width,row.canvas.height);c.save();c.scale(2,2);c.fillStyle='rgba(0,0,0,.28)';for(const x of [...(data.baseline?[previousSize*.5]:[]),previousSize+currentSize*data.originX]){c.beginPath();c.ellipse(x,baseline,1.1*720/14/2,1.1*720/14/4,0,0,Math.PI*2);c.fill();}
  if(data.baseline)c.drawImage(idle?row.beforeIdle:row.before[Math.floor((previousTravel%data.baseline.stride)/data.baseline.stride*data.baseline.frames)%data.baseline.frames],0,baseline-previousSize*data.baseline.originY,previousSize,previousSize);c.drawImage(idle?row.idle:row.after[frame],previousSize,baseline-currentSize*data.originY,currentSize,currentSize);c.restore();}
 requestAnimationFrame(draw);
}));
</script>`,
);
console.log(resolve(output, "index.html"));
