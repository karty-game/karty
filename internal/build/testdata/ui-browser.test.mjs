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
      globalThis.uiObserved = { values:{}, nodes:{}, shows:0, commands:0 };
      Object.defineProperty(globalThis, "kartyHostSubmitCommands", {
        configurable:true,
        get: () => submit,
        set: original => { submit = bytes => {
          const data = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
          for (let offset=24; offset<bytes.length;) {
            const tag=data.getUint8(offset), length=data.getUint16(offset+1,true), start=offset+3;
            const text = (from, count) => new TextDecoder().decode(bytes.subarray(from,from+count));
            if (tag===18) { uiObserved.shows++; uiObserved.asset=text(start+10,data.getUint16(start+8,true)); uiObserved.values={}; uiObserved.nodes={}; }
            if (tag===23) uiObserved.nodes[data.getUint32(start,true)]={};
            if (tag===25) delete uiObserved.nodes[data.getUint32(start,true)];
            if (tag===20 && uiObserved.nodes[data.getUint32(start,true)]) uiObserved.nodes[data.getUint32(start,true)][data.getUint32(start+4,true)]=text(start+11,data.getUint16(start+9,true));
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
    const click = async y => {
      const frame = await page.locator("html").getAttribute("data-karty-frame");
      await page.waitForFunction(frame=>BigInt(document.documentElement.dataset.kartyFrame)>BigInt(frame)+2n,frame);
      if (mobile) {
        await cdp.send("Input.dispatchTouchEvent",{type:"touchStart",touchPoints:[{x:box.x+60,y:box.y+y}]});
        await page.waitForTimeout(100);
        await cdp.send("Input.dispatchTouchEvent",{type:"touchEnd",touchPoints:[]});
      } else await page.mouse.click(box.x+60,box.y+y,{delay:100});
    };
    await canvas.screenshot({path:`/tmp/karty-ui-${mobile?"phone":"desktop"}.png`});
    const dimensions = await canvas.evaluate(canvas=>({width:canvas.width,height:canvas.height,cssWidth:canvas.getBoundingClientRect().width}));
    assert.ok(dimensions.width >= dimensions.cssWidth*(mobile?1.9:0.9), "UI is not using display-density pixels");
    // Navigate the current scaffold by focus order; theme spacing is not an API.
    const activate = async (tabs=0, reverse=false) => {
      await page.waitForTimeout(150);
      for (let i=0;i<tabs;i++) await page.keyboard.press(reverse?"Shift+Tab":"Tab",{delay:100});
      await page.keyboard.press("Enter",{delay:100});
    };
    await activate();
    await page.waitForFunction(() => uiObserved.asset==="ui.levels");
    await page.route("**/*.kld", route => route.fulfill({status:404,body:"intentional missing level"}));
    await activate();
    await page.waitForFunction(() => uiObserved.asset==="ui.error");
    await page.unroute("**/*.kld");
    await activate();
    await page.waitForFunction(() => uiObserved.asset==="ui.hud");
    await click(310);
    await page.waitForFunction(() => uiObserved.asset==="ui.pause");
    await activate(1);
    await page.waitForFunction(() => uiObserved.asset==="ui.inventory");
    await canvas.screenshot({path:`/tmp/karty-ui-inventory-${mobile?"phone":"desktop"}.png`});
    await activate(4);
    await page.waitForFunction(() => Object.values(uiObserved.nodes).some(values=>Object.values(values).some(text=>text.includes("used 1 times"))));
    await page.waitForFunction(() => Object.values(uiObserved.nodes).some(values=>Object.values(values).some(text=>text.includes("2 left"))));
    const before = await page.evaluate(()=>uiObserved.commands);
    const frame = await page.locator("html").getAttribute("data-karty-frame");
    await page.waitForFunction(frame=>BigInt(document.documentElement.dataset.kartyFrame)>BigInt(frame)+10n,frame);
    assert.equal(await page.evaluate(()=>uiObserved.commands),before,"idle UI emitted commands");
    await activate(4,true);
    await page.waitForFunction(() => uiObserved.asset==="ui.pause");
    await activate(1);
    await page.waitForFunction(() => uiObserved.asset==="ui.inventory");
    await page.waitForFunction(() => Object.values(uiObserved.nodes).some(values=>Object.values(values).some(text=>text.includes("used 0 times"))));
    await activate();
    await page.waitForFunction(() => uiObserved.asset==="ui.pause");
    await activate(2);
    await page.waitForFunction(() => uiObserved.asset==="ui.menu");
    await activate();
    await page.waitForFunction(() => uiObserved.asset==="ui.levels");
    await activate(1);
    await page.waitForFunction(() => uiObserved.asset==="ui.hud");
    await click(310);
    await page.waitForFunction(() => uiObserved.asset==="ui.pause");
    await activate(2);
    await page.waitForFunction(() => uiObserved.asset==="ui.menu");
    await page.setViewportSize(mobile?{width:844,height:390}:{width:640,height:480});
    await page.waitForTimeout(100);
    assert.ok((await canvas.screenshot()).length>1024,"UI disappeared on resize");
    assert.deepEqual(errors,[]);
    await context.close();
  }
  console.log("UI browser: desktop and high-DPI primary touch lifecycle passed");
} finally { await browser.close(); await new Promise(resolve=>server.close(resolve)); }
