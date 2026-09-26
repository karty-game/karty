// Executes the real EbitenUI host and TinyGo UI scaffold. Transport observation
// lives exclusively in this test; production needs no test/debug UI API.
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { resolve, relative, extname } from "node:path";
import { chromium } from "playwright";

const root = resolve(process.argv[2]);
const server = createServer(async (request, response) => {
  try {
    const path = resolve(root, "." + new URL(request.url, "http://test").pathname.replace(/^\/proxy\/4242/, ""),);
    if (relative(root, path).startsWith("..")) { response.writeHead(403); response.end(); return; }
    const file = path === root ? resolve(root, "index.html") : path;
    const bytes = await readFile(file);
    const mime = { ".html": "text/html", ".js": "text/javascript", ".wasm": "application/wasm", ".kart": "application/wasm", ".kld": "application/wasm" };
    response.writeHead(200, { "Content-Type": mime[extname(file)] || "application/octet-stream" });
    response.end(bytes);
  } catch { response.writeHead(404); response.end(); }
});
await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
const browser = await chromium.launch({ headless: true });
try {
  for (const mobile of [false, true]) {
    const context = await browser.newContext({ viewport: mobile ? { width:390, height:844 } : { width:960, height:640 }, deviceScaleFactor:mobile?2:1, hasTouch:mobile, isMobile:mobile, reducedMotion:mobile?"reduce":"no-preference" });
    const page = await context.newPage();
    const errors = [];
    page.on("pageerror", error => errors.push(error.message));
    const logs = [];
    page.on("console", message => { logs.push(message.text()); if (logs.length>40) logs.shift(); });
    await page.addInitScript(() => {
      let submit;
      globalThis.uiObserved = { values:{}, nodes:{}, order:[], visibility:{}, shows:0, commands:0 };
      Object.defineProperty(globalThis, "kartyHostSubmitCommands", {
        configurable:true,
        get: () => submit,
        set: original => { submit = bytes => {
          const data = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
          for (let offset=24; offset<bytes.length;) {
            const tag=data.getUint8(offset), length=data.getUint16(offset+1,true), start=offset+3;
            const text = (from, count) => new TextDecoder().decode(bytes.subarray(from,from+count));
            if (tag===18) { uiObserved.shows++; uiObserved.asset=text(start+10,data.getUint16(start+8,true)); uiObserved.values={}; }
            if (tag===23) { const id=data.getUint32(start,true); uiObserved.nodes[id]={}; uiObserved.order.splice(data.getUint32(start+12,true),0,id); }
            if (tag===24) { const id=data.getUint32(start,true); uiObserved.order.splice(uiObserved.order.indexOf(id),1); uiObserved.order.splice(data.getUint32(start+4,true),0,id); }
            if (tag===25) { const id=data.getUint32(start,true); delete uiObserved.nodes[id]; uiObserved.order=uiObserved.order.filter(x=>x!==id); }
            if (tag===26) uiObserved.visibility[data.getUint32(start+4,true)] = data.getUint8(start+8)===1;
            if (tag===20 && uiObserved.nodes[data.getUint32(start,true)]) uiObserved.nodes[data.getUint32(start,true)][data.getUint32(start+4,true)] = text(start+11,data.getUint16(start+9,true));
            if (tag===20) uiObserved.values[data.getUint32(start+4,true)] = text(start+11,data.getUint16(start+9,true));
            if (tag===21) uiObserved.row = text(start+19,data.getUint16(start+17,true));
            uiObserved.commands++;
            offset=start+length;
          }
          return original(bytes);
        }; }
      });
    });
    await page.goto(`http://127.0.0.1:${server.address().port}/proxy/4242/`);
    try {
      await page.waitForFunction(() => document.documentElement.dataset.kartyState === "running", null, {timeout:30000});
    } catch (error) {
      throw new Error(`UI startup failed: ${await page.locator("#error-detail").textContent()}; ${errors.join("; ")}; ${logs.join("\n")}; ${error.message}`);
    }
    const canvas = page.locator("canvas");
    await canvas.waitFor({state:"visible"});
    await page.waitForFunction(() => uiObserved.asset==="ui.menu");
    const box = await canvas.boundingBox();
    assert.ok(box);
    const cdp = await context.newCDPSession(page);
    const click = async (y, xFraction=1/3) => {
      const frame = await page.locator("html").getAttribute("data-karty-frame");
      await page.waitForFunction(frame=>BigInt(document.documentElement.dataset.kartyFrame)>BigInt(frame)+2n,frame);
      // One-third remains inside full-width controls and intrinsic child buttons
      // while still exercising centered parent layouts on desktop and touch.
      const x=box.x+box.width*xFraction;
      if (mobile) {
        await cdp.send("Input.dispatchTouchEvent",{type:"touchStart",touchPoints:[{x,y:box.y+y}]});
        await page.waitForTimeout(100);
        await cdp.send("Input.dispatchTouchEvent",{type:"touchEnd",touchPoints:[]});
      } else await page.mouse.click(x,box.y+y,{delay:100});
    };
    const drag = async (fromY, toY) => {
      const x=box.x+box.width/3;
      await cdp.send("Input.dispatchTouchEvent",{type:"touchStart",touchPoints:[{x,y:box.y+fromY}]});
      for (let step=1; step<=4; step++) {
        const y=fromY+(toY-fromY)*step/4;
        await cdp.send("Input.dispatchTouchEvent",{type:"touchMove",touchPoints:[{x,y:box.y+y}]});
        await page.waitForTimeout(50);
      }
      await cdp.send("Input.dispatchTouchEvent",{type:"touchEnd",touchPoints:[]});
    };

    await canvas.screenshot({path:"/tmp/karty-theme-menu-"+(mobile?"phone":"desktop")+".png"});
    // SDK 0.10 starts with a visible default focus; pointer activation remains
    // available without requiring the browser canvas to own DOM keyboard focus.
    await click(mobile ? 190 : 250, mobile ? 1/2 : 1/5);
    await page.waitForFunction(() => uiObserved.asset==="ui.inventory" && uiObserved.order.length===2);
    await canvas.screenshot({path:"/tmp/karty-theme-inventory-"+(mobile?"phone":"desktop")+".png"});
    await page.waitForTimeout(250);
    if (mobile) {
      for (let expected=3; expected<=5; expected++) {
        await click(132);
        await page.waitForFunction(expected=>uiObserved.order.length===expected,expected);
      }
      await page.waitForTimeout(250);
      const commandsBeforeScroll=await page.evaluate(()=>uiObserved.commands);
      await drag(390,275);
      await page.waitForTimeout(100);
      assert.equal(await page.evaluate(()=>uiObserved.commands),commandsBeforeScroll,"touch scroll emitted client commands or activated a control");
      await canvas.screenshot({path:"/tmp/karty-touch-scroll-phone.png"});
      const scrollTarget=await page.evaluate(()=>uiObserved.order[2]);
      await click(365,1/2);
      await page.waitForFunction(id=>Object.values(uiObserved.nodes[id]).some(value=>value?.includes("used 1 times")),scrollTarget);
      await canvas.screenshot({path:"/tmp/karty-touch-scroll-preserved-phone.png"});
      await click(365,1/2);
      await page.waitForFunction(id=>Object.values(uiObserved.nodes[id]).some(value=>value?.includes("used 2 times")),scrollTarget);
      for (let expected=4; expected>=2; expected--) {
        await click(178);
        await page.waitForFunction(expected=>uiObserved.order.length===expected,expected);
      }
    }
    const original = await page.evaluate(() => [...uiObserved.order]);
    await click(mobile ? 300 : 407,mobile ? 1/2 : 1/3);
    await page.waitForFunction(() => Object.values(uiObserved.nodes).some(node=>Object.values(node).some(value=>value?.includes("used 1 times"))));
    const used = await page.evaluate(() => uiObserved.order.find(id=>Object.values(uiObserved.nodes[id]).some(value=>value?.includes("used 1 times"))));
    await click(mobile ? 224 : 307);
    await page.waitForFunction(first=>uiObserved.order[1]===first,original[0]);
    assert.equal(await page.evaluate(id=>Object.values(uiObserved.nodes[id]).some(value=>value?.includes("used 1 times")),used),true);
    await click(mobile ? 132 : 190);
    await page.waitForFunction(() => uiObserved.order.length===3);
    await click(mobile ? 178 : 250);
    await page.waitForFunction(() => uiObserved.order.length===2);
    assert.equal(await page.evaluate(id=>!!uiObserved.nodes[id],original[1]),false);
    assert.equal(await page.evaluate(id=>Object.values(uiObserved.nodes[id]).some(value=>value?.includes("used 1 times")),used),true);
	await click(mobile ? 178 : 250);
	await page.waitForFunction(() => uiObserved.order.length===1);
	await click(mobile ? 178 : 250);
	await page.waitForFunction(() => uiObserved.order.length===0 && uiObserved.visibility[9]===true && uiObserved.visibility[10]===false);
	await canvas.screenshot({path:"/tmp/karty-conditional-empty-"+(mobile?"phone":"desktop")+".png"});
	await click(mobile ? 132 : 190);
	await page.waitForFunction(() => uiObserved.order.length===1 && uiObserved.visibility[9]===false && uiObserved.visibility[10]===true);
    const commandCount=await page.evaluate(()=>uiObserved.commands);
    await page.waitForTimeout(200);
    assert.equal(await page.evaluate(()=>uiObserved.commands),commandCount,"idle UI emitted commands");
    await canvas.screenshot({path:"/tmp/karty-composition-"+(mobile?"phone":"desktop")+".png"});
    await page.keyboard.press("Escape");
    await page.waitForFunction(() => uiObserved.asset==="ui.menu");
    assert.deepEqual(errors,[]);
    await context.close();
  }
  console.log("Keyed composition: desktop/touch add, remove, reorder, local state and idle passed");
} finally { await browser.close(); await new Promise(resolve=>server.close(resolve)); }
