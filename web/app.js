"use strict";

const byId = (id) => document.getElementById(id);
const connection = byId("connection");
const menu = byId("connection-menu");
const controls = byId("controls");
const pairScreen = byId("pair");
const pairForm = byId("pair-form");
const pairCode = byId("pair-code");
const pairMessage = byId("pair-message");
const tabs = byId("tabs");
const toast = byId("toast");
const trackpad = byId("trackpad");
const dragButton = byId("drag");
const textBox = byId("text-box");
const live = byId("live");
const sendText = byId("send-text");

let socket;
let paired = false;
let nextID = 1;
let heartbeat;
let reconnectTimer;
let reconnectDelay = 350;
let lastReply = 0;
let toastTimer;
let activeTab = "streaming";
let dragging = false;
let changingDrag = false;
const pending = new Map();

function notify(message) {
  toast.textContent = message;
  toast.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { toast.hidden = true; }, 2800);
}

function connectionState(state, label) {
  connection.dataset.state = state;
  connection.querySelector("b").textContent = label;
  connection.disabled = !paired;
}

function hideMenu() {
  menu.hidden = true;
  connection.setAttribute("aria-expanded", "false");
}

connection.addEventListener("click", () => {
  menu.hidden = !menu.hidden;
  connection.setAttribute("aria-expanded", String(!menu.hidden));
});
document.addEventListener("click", (event) => {
  if (!menu.hidden && !menu.contains(event.target) && !connection.contains(event.target)) hideMenu();
});
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && !menu.hidden) {
    hideMenu();
    connection.focus();
  }
});
byId("host-address").textContent = location.host;

function showPair(message = "") {
  paired = false;
  controls.hidden = true;
  tabs.hidden = true;
  pairScreen.hidden = false;
  pairMessage.textContent = message;
  connectionState("offline", "Not paired");
  hideMenu();
  clearTimeout(reconnectTimer);
  stopSocket();
}

function showControls() {
  paired = true;
  pairScreen.hidden = true;
  controls.hidden = false;
  tabs.hidden = false;
  connectionState("retry", "Connecting");
  connect();
}

async function fetchState() {
  const response = await fetch("/api/state");
  if (!response.ok) throw new Error("Unable to reach your PC.");
  return (await response.json()).paired;
}

async function submitPair(body) {
  const response = await fetch("/api/pair", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) throw new Error((await response.text()).trim());
}

pairForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const button = pairForm.querySelector("button");
  button.disabled = true;
  pairMessage.textContent = "";
  try {
    await submitPair({ pin: pairCode.value.trim() });
    pairForm.reset();
    showControls();
  } catch (error) {
    pairMessage.textContent = error.message || "Unable to reach your PC.";
  } finally {
    button.disabled = false;
  }
});
pairCode.addEventListener("input", () => {
  pairCode.value = pairCode.value.replace(/\D/g, "").slice(0, 6);
  pairMessage.textContent = "";
});

byId("disconnect").addEventListener("click", async () => {
  try {
    const response = await fetch("/api/disconnect", { method: "POST" });
    if (!response.ok) throw new Error("Couldn't disconnect. Try again.");
    showPair();
  } catch (error) {
    notify(error.message);
  }
});

function clearPending() {
  for (const request of pending.values()) {
    clearTimeout(request.timeout);
    request.reject(new Error("Connection lost. Try again when your PC reconnects."));
  }
  pending.clear();
}

function stopSocket() {
  clearInterval(heartbeat);
  const old = socket;
  socket = undefined;
  if (old) old.close();
  clearPending();
  applyDrag(false);
  resetLiveBuffer();
}

function reconnect() {
  clearTimeout(reconnectTimer);
  if (!paired || document.hidden) return;
  connectionState("retry", "Reconnecting");
  reconnectTimer = setTimeout(async () => {
    try {
      const authorized = await fetchState();
      if (!paired || document.hidden) return;
      if (!authorized) {
        showPair("Pair again using the code on your PC.");
        return;
      }
      connect();
    } catch {
      reconnect();
    }
  }, reconnectDelay);
  reconnectDelay = Math.min(reconnectDelay * 1.65, 4500);
}

function connect() {
  if (!paired || document.hidden || socket) return;
  const client = new WebSocket(`ws://${location.host}/api/control`);
  socket = client;
  client.addEventListener("open", () => {
    if (client !== socket) return;
    reconnectDelay = 350;
    lastReply = Date.now();
    connectionState("online", "Connected");
    heartbeat = setInterval(() => {
      if (Date.now() - lastReply > 12500) {
        client.close();
        return;
      }
      fire({ kind: "ping" });
    }, 4000);
  });
  client.addEventListener("message", ({ data }) => {
    if (client !== socket) return;
    lastReply = Date.now();
    const result = JSON.parse(data);
    const request = pending.get(result.id);
    if (!request) return;
    pending.delete(result.id);
    clearTimeout(request.timeout);
    if (result.error) request.reject(new Error(result.error));
    else request.resolve();
  });
  client.addEventListener("close", ({ code }) => {
    if (client !== socket) return;
    stopSocket();
    if (code === 4001) showPair("Pair again using the code on your PC.");
    else reconnect();
  });
}

function fire(command) {
  if (socket?.readyState !== WebSocket.OPEN || socket.bufferedAmount > 32768) return false;
  socket.send(JSON.stringify(command));
  return true;
}

function request(command) {
  return new Promise((resolve, reject) => {
    const id = nextID++;
    if (!fire({ ...command, id })) {
      reject(new Error("Waiting for a connection to your PC."));
      return;
    }
    const timeout = setTimeout(() => {
      pending.delete(id);
      reject(new Error("Your PC didn't confirm the action."));
    }, 5000);
    pending.set(id, { resolve, reject, timeout });
  });
}

document.addEventListener("visibilitychange", () => {
  if (!paired) return;
  if (document.hidden) stopSocket();
  else reconnect();
});
window.addEventListener("pagehide", stopSocket);
window.addEventListener("pageshow", (event) => {
  if (event.persisted && paired) reconnect();
});

const serviceGrid = byId("service-grid");
const serviceCount = byId("service-count");
const search = byId("search");
let serviceList = [];

function renderServices() {
  const query = search.value.trim().toLowerCase();
  const matches = serviceList.filter((service) => service.name.toLowerCase().includes(query));
  serviceGrid.replaceChildren();
  serviceCount.textContent = matches.length === 1 ? "1 service" : `${matches.length} services`;
  byId("empty-services").hidden = matches.length !== 0;
  for (const service of matches) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "service-card";
    button.setAttribute("aria-label", `Open ${service.name} on your PC`);
    const cover = document.createElement("span");
    cover.className = `service-art ${service.id}`;
    cover.setAttribute("aria-hidden", "true");
    cover.textContent = service.mark;
    const label = document.createElement("span");
    label.className = "service-label";
    const icon = document.createElement("i");
    icon.dataset.icon = "arrow-up-right";
    label.append(service.name, icon);
    button.append(cover, label);
    button.addEventListener("click", async () => {
      if (button.getAttribute("aria-busy") === "true") return;
      button.setAttribute("aria-busy", "true");
      try {
        await request({ kind: "open", value: service.id });
        notify(`Opened ${service.name} on your PC.`);
      } catch (error) {
        notify(error.message);
      } finally {
        button.removeAttribute("aria-busy");
      }
    });
    serviceGrid.append(button);
  }
}

search.addEventListener("input", renderServices);

function selectTab(tab, focus = false) {
  if (!["streaming", "mouse", "keyboard", "remote"].includes(tab)) return;
  if (activeTab === "mouse" && tab !== "mouse") releaseDrag();
  if (activeTab === "keyboard" && tab !== "keyboard") resetLiveBuffer();
  activeTab = tab;
  for (const panel of controls.querySelectorAll(".panel")) panel.hidden = panel.id !== tab;
  for (const button of tabs.querySelectorAll("button")) {
    if (button.dataset.tab === tab) button.setAttribute("aria-current", "page");
    else button.removeAttribute("aria-current");
  }
  window.scrollTo(0, 0);
  if (focus) {
    const heading = byId(`${tab}-title`);
    heading.tabIndex = -1;
    heading.focus({ preventScroll: true });
  }
}

tabs.addEventListener("click", (event) => {
  const tab = event.target.closest("[data-tab]");
  if (tab) selectTab(tab.dataset.tab, event.detail === 0);
});

function applyDrag(value) {
  dragging = value;
  dragButton.setAttribute("aria-pressed", String(value));
  dragButton.querySelector("span").textContent = value ? "Release" : "Drag";
}

function releaseDrag() {
  if (!dragging && !changingDrag) return;
  fire({ kind: "drag", down: false });
  applyDrag(false);
}

dragButton.addEventListener("click", async () => {
  if (changingDrag) return;
  changingDrag = true;
  const down = !dragging;
  try {
    await request({ kind: "drag", down });
    if (activeTab === "mouse" && !document.hidden) applyDrag(down);
    else if (down) fire({ kind: "drag", down: false });
  } catch (error) {
    notify(error.message);
  } finally {
    changingDrag = false;
  }
});

for (const button of document.querySelectorAll("[data-click]")) {
  button.addEventListener("click", () => {
    request({ kind: "click", button: button.dataset.click }).catch((error) => notify(error.message));
  });
}

const points = new Map();
const sensitivity = byId("sensitivity");
let gesture;
let frame = 0;
let moveX = 0;
let moveY = 0;
let wheel = 0;

function flushMotion() {
  cancelAnimationFrame(frame);
  frame = 0;
  const x = Math.trunc(moveX);
  const y = Math.trunc(moveY);
  const amount = Math.trunc(wheel);
  moveX -= x;
  moveY -= y;
  wheel -= amount;
  if (x || y) fire({ kind: "move", x, y });
  if (amount) fire({ kind: "scroll", amount });
}

function scheduleMotion() {
  if (!frame) frame = requestAnimationFrame(flushMotion);
}

trackpad.addEventListener("pointerdown", (event) => {
  if (event.button !== 0) return;
  event.preventDefault();
  trackpad.setPointerCapture(event.pointerId);
  points.set(event.pointerId, { x: event.clientX, y: event.clientY });
  trackpad.dataset.active = "true";
  if (points.size === 1) {
    gesture = { x: event.clientX, y: event.clientY, started: performance.now(), moved: false, multi: false };
  } else {
    gesture.multi = true;
  }
});

trackpad.addEventListener("pointermove", (event) => {
  const old = points.get(event.pointerId);
  if (!old) return;
  const pt = { x: event.clientX, y: event.clientY };
  points.set(event.pointerId, pt);
  const dx = pt.x - old.x;
  const dy = pt.y - old.y;
  if (points.size > 1) {
    wheel += dy * 7 / points.size;
    scheduleMotion();
    return;
  }
  if (gesture.multi) return;
  const speed = Number(sensitivity.value);
  if (!gesture.moved) {
    const x = pt.x - gesture.x;
    const y = pt.y - gesture.y;
    if (Math.hypot(x, y) < 5) return;
    gesture.moved = true;
    moveX += x * speed;
    moveY += y * speed;
  } else {
    moveX += dx * speed;
    moveY += dy * speed;
  }
  scheduleMotion();
});

function endPointer(event) {
  if (!points.delete(event.pointerId)) return;
  if (event.type !== "pointerup") {
    gesture.multi = true;
    releaseDrag();
  }
  flushMotion();
  if (points.size) return;
  trackpad.dataset.active = "false";
  if (!gesture.moved && !gesture.multi && !dragging && performance.now() - gesture.started < 420) {
    request({ kind: "click", button: "left" }).catch((error) => notify(error.message));
  }
  gesture = undefined;
}

for (const name of ["pointerup", "pointercancel", "lostpointercapture"]) trackpad.addEventListener(name, endPointer);
trackpad.addEventListener("contextmenu", (event) => event.preventDefault());
trackpad.addEventListener("wheel", (event) => {
  event.preventDefault();
  const scale = event.deltaMode === 1 ? 36 : event.deltaMode === 2 ? 240 : 3;
  wheel -= event.deltaY * scale;
  scheduleMotion();
}, { passive: false });

let draft = "";
let baseline = "";
let liveEpoch = 0;
let updatingLive = false;
let composing = false;
let sendingText = false;
const segmenter = typeof Intl.Segmenter === "function" ? new Intl.Segmenter(undefined, { granularity: "grapheme" }) : null;
const characters = (text) => segmenter ? Array.from(segmenter.segment(text), ({ segment }) => segment) : Array.from(text);

function showTypingState() {
  const liveMode = live.checked;
  sendText.hidden = liveMode;
  sendText.disabled = sendingText || !textBox.value;
  byId("typing-help").textContent = liveMode
    ? "Text appears on your PC as you type."
    : "Write a message, then tap Send text.";
}

function resetLiveBuffer() {
  if (!live.checked) return;
  liveEpoch++;
  baseline = "";
  composing = false;
  textBox.value = "";
}

live.addEventListener("change", () => {
  if (live.checked) {
    draft = textBox.value;
    resetLiveBuffer();
  } else {
    liveEpoch++;
    composing = false;
    textBox.value = draft;
  }
  showTypingState();
});

async function updateLive() {
  if (!live.checked || composing || updatingLive || baseline === textBox.value) return;
  const value = textBox.value;
  const previous = characters(baseline);
  const current = characters(value);
  let keep = 0;
  while (keep < previous.length && keep < current.length && previous[keep] === current[keep]) keep++;
  const epoch = liveEpoch;
  updatingLive = true;
  try {
    await request({ kind: "type", value: current.slice(keep).join(""), erase: previous.length - keep });
    if (epoch === liveEpoch) baseline = value;
  } catch (error) {
    if (epoch === liveEpoch) {
      // The PC may have processed an unacknowledged edit. Start a fresh buffer.
      resetLiveBuffer();
      notify(error.message);
    }
  } finally {
    updatingLive = false;
    if (live.checked && baseline !== textBox.value) updateLive();
  }
}

textBox.addEventListener("compositionstart", () => { composing = true; });
textBox.addEventListener("compositionend", () => {
  composing = false;
  updateLive();
});
textBox.addEventListener("input", (event) => {
  showTypingState();
  if (!event.isComposing) updateLive();
});
sendText.addEventListener("click", async () => {
  const value = textBox.value;
  if (!value || sendingText || live.checked) return;
  sendingText = true;
  showTypingState();
  try {
    await request({ kind: "type", value });
    if (!live.checked && textBox.value === value) textBox.value = "";
    if (live.checked && draft === value) draft = "";
    notify("Text sent to your PC.");
  } catch (error) {
    notify(error.message);
  } finally {
    sendingText = false;
    showTypingState();
  }
});

for (const button of document.querySelectorAll("[data-key]")) {
  button.addEventListener("click", () => {
    resetLiveBuffer();
    request({ kind: "key", value: button.dataset.key }).catch((error) => notify(error.message));
  });
}

async function initialize() {
  const secret = new URLSearchParams(location.hash.slice(1)).get("pair");
  if (secret) history.replaceState(null, "", location.pathname + location.search);
  try {
    if (secret) await submitPair({ secret });
    if (await fetchState()) showControls();
    else showPair();
  } catch (error) {
    showPair(error.message || "Unable to reach your PC.");
  }
  try {
    const response = await fetch("/services.json");
    if (!response.ok) throw new Error();
    serviceList = await response.json();
    renderServices();
  } catch {
    byId("empty-services").textContent = "Services couldn't load. Reload this page to try again.";
    byId("empty-services").hidden = false;
  }
}

initialize();
