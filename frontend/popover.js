import { Call, Events } from "/wails/runtime.js";

const call = (method, ...args) => Call.ByName(`main.GifService.${method}`, ...args);
const $ = (id) => document.getElementById(id);
const isMac = /Mac/.test(navigator.platform);

let state = null;

function prettyHotkey(accel) {
  return accel
    .split("+")
    .map((k) => {
      const l = k.toLowerCase();
      if (l === "cmdorctrl") return isMac ? "⌘" : "Ctrl";
      if (l === "shift") return isMac ? "⇧" : "Shift";
      if (l === "optionoralt" || l === "alt") return isMac ? "⌥" : "Alt";
      return k.toUpperCase();
    })
    .join(isMac ? "" : "+");
}

function clock(sec) {
  const s = Math.floor(sec);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
}

function size(bytes) {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function when(ms) {
  const d = new Date(ms);
  const t = d.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
  const today = new Date();
  if (d.toDateString() === today.toDateString()) return t;
  const y = new Date(today); y.setDate(today.getDate() - 1);
  if (d.toDateString() === y.toDateString()) return "Yesterday";
  return d.toLocaleDateString([], { month: "short", day: "numeric" });
}

let toastTimer;
function toast(msg) {
  const t = $("toast");
  t.textContent = msg;
  t.classList.add("show");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => t.classList.remove("show"), 1400);
}

let selectedMode = "region";

function updateModeUI() {
  document.querySelectorAll(".mode-card").forEach((card) => {
    card.classList.toggle("active", card.dataset.mode === selectedMode);
  });
  const label = $("start-btn-label");
  if (!label) return;
  switch (selectedMode) {
    case "window":
      label.textContent = "Start Window Recording";
      break;
    case "region":
      label.textContent = "Start Region Recording";
      break;
    case "screen":
      label.textContent = "Start Full Screen Recording";
      break;
    case "last":
      label.textContent = "Start Same Area Recording";
      break;
    default:
      label.textContent = "Start Recording";
  }
}

// ---- state ----

function render(s) {
  state = s;
  document.querySelectorAll("[data-show]").forEach((el) => {
    el.hidden = !el.dataset.show.split(" ").includes(s.phase === "picking" ? "idle" : s.phase);
  });

  const permBanner = $("perm-banner");
  if (permBanner) {
    permBanner.hidden = s.hasPerm !== false;
  }

  const lastCard = $("last");
  if (lastCard) {
    lastCard.disabled = !s.hasLast;
    if (selectedMode === "last" && !s.hasLast) {
      selectedMode = "region";
      updateModeUI();
    }
  }

  if ($("hotkey")) $("hotkey").textContent = prettyHotkey(s.hotkey);
  if ($("hotkey2")) $("hotkey2").textContent = prettyHotkey(s.hotkey);
  $("error").hidden = !s.error;
  $("error").textContent = s.error || "";

  const livePhase = $("live-phase");
  const liveDot = $("live-dot");
  const popoverPauseBtn = $("popover-pause-btn");
  if (s.phase === "countdown") {
    if (livePhase) livePhase.textContent = "STARTING…";
    $("timer").textContent = "3…";
    if (popoverPauseBtn) popoverPauseBtn.style.display = "none";
    if (liveDot) liveDot.classList.remove("paused");
  } else if (s.phase === "recording") {
    if (livePhase) livePhase.textContent = "RECORDING";
    $("timer").textContent = clock(s.elapsed);
    if (popoverPauseBtn) {
      popoverPauseBtn.style.display = "inline-flex";
      popoverPauseBtn.textContent = "Pause";
    }
    if (liveDot) liveDot.classList.remove("paused");
  } else if (s.phase === "paused") {
    if (livePhase) livePhase.textContent = "PAUSED";
    $("timer").textContent = clock(s.elapsed);
    if (popoverPauseBtn) {
      popoverPauseBtn.style.display = "inline-flex";
      popoverPauseBtn.textContent = "Resume";
    }
    if (liveDot) liveDot.classList.add("paused");
  }
  if (s.phase === "encoding") setPct(0);
  if (s.phase === "review") {
    if (!reviewInfo) setupReview();
  } else {
    stopPlayback();
    reviewInfo = null;
  }
  renderSettings(s.settings);
}

function setPct(p) {
  $("pct").textContent = `${p}%`;
  $("bar").style.width = `${p}%`;
}

// ---- review / trim editor ----

let reviewInfo = null;
let currentPlayhead = 0;
let isPlaying = false;
let playTimer = null;

async function setupReview(info) {
  if (!info) {
    try {
      info = await call("GetReviewInfo");
    } catch (e) {
      return;
    }
  }
  if (!info || info.numFrames <= 0) return;
  reviewInfo = info;

  const startSlider = $("trim-start");
  const endSlider = $("trim-end");
  if (startSlider && endSlider) {
    startSlider.min = 0;
    startSlider.max = info.numFrames - 1;
    startSlider.value = 0;

    endSlider.min = 0;
    endSlider.max = info.numFrames - 1;
    endSlider.value = info.numFrames - 1;
  }

  currentPlayhead = 0;
  stopPlayback();
  updateTimelineUI();
  showFrame(0);
}

function showFrame(idx) {
  if (!reviewInfo) return;
  idx = Math.max(0, Math.min(idx, (reviewInfo.numFrames || 1) - 1));
  currentPlayhead = idx;
  const img = $("review-img");
  if (img) {
    img.src = `/preview/frame?i=${idx}&t=${Date.now()}`;
  }
  const badge = $("review-frame-badge");
  if (badge) {
    badge.textContent = `${idx + 1} / ${reviewInfo.numFrames}`;
  }
  updateNeedle();
}

function updateTimelineUI() {
  if (!reviewInfo) return;
  const total = Math.max(1, reviewInfo.numFrames - 1);
  const startSlider = $("trim-start");
  const endSlider = $("trim-end");
  if (!startSlider || !endSlider) return;

  const startVal = Number(startSlider.value);
  const endVal = Number(endSlider.value);

  const startPct = (startVal / total) * 100;
  const endPct = (endVal / total) * 100;

  const rangeEl = $("timeline-range");
  if (rangeEl) {
    rangeEl.style.left = `${startPct}%`;
    rangeEl.style.width = `${Math.max(0, endPct - startPct)}%`;
  }

  const selectedCount = endVal - startVal + 1;
  const fps = Math.max(1, reviewInfo.fps || 15);
  const trimmedSec = (selectedCount / fps).toFixed(1);
  const totalSec = reviewInfo.duration ? reviewInfo.duration.toFixed(1) : (reviewInfo.numFrames / fps).toFixed(1);

  if ($("review-dur")) $("review-dur").textContent = `${trimmedSec}s / ${totalSec}s`;
  if ($("review-frames")) $("review-frames").textContent = `${selectedCount} of ${reviewInfo.numFrames} frames`;

  updateNeedle();
}

function updateNeedle() {
  if (!reviewInfo) return;
  const total = Math.max(1, reviewInfo.numFrames - 1);
  const pct = (currentPlayhead / total) * 100;
  const needle = $("timeline-needle");
  if (needle) {
    needle.style.left = `${pct}%`;
  }
}

function togglePlayback() {
  if (isPlaying) {
    stopPlayback();
  } else {
    startPlayback();
  }
}

function startPlayback() {
  if (!reviewInfo || reviewInfo.numFrames <= 0) return;
  isPlaying = true;
  $("play-icon")?.setAttribute("hidden", "true");
  $("pause-icon")?.removeAttribute("hidden");

  const startVal = Number($("trim-start")?.value || 0);
  const endVal = Number($("trim-end")?.value || 0);
  if (currentPlayhead < startVal || currentPlayhead >= endVal) {
    currentPlayhead = startVal;
  }

  const fps = Math.max(1, reviewInfo.fps || 15);
  const interval = Math.round(1000 / fps);

  clearInterval(playTimer);
  playTimer = setInterval(() => {
    const s = Number($("trim-start")?.value || 0);
    const e = Number($("trim-end")?.value || 0);
    currentPlayhead++;
    if (currentPlayhead > e) {
      currentPlayhead = s;
    }
    showFrame(currentPlayhead);
  }, interval);
}

function stopPlayback() {
  isPlaying = false;
  $("play-icon")?.removeAttribute("hidden");
  $("pause-icon")?.setAttribute("hidden", "true");
  clearInterval(playTimer);
}

// ---- library ----

async function loadLibrary(highlight) {
  const items = await call("ListRecordings");
  const grid = $("grid");
  grid.replaceChildren();
  $("empty").hidden = items.length > 0;
  for (const r of items) grid.append(card(r, r.name === highlight));
}

function card(r, fresh) {
  const el = document.createElement("div");
  el.className = "item";
  el.innerHTML = `
    <div class="pic" title="Click to copy, double-click to open">
      <img alt="" draggable="false">
      <div class="acts">
        <button data-a="copy">Copy</button>
        <button data-a="reveal" title="${isMac ? "Show in Finder" : "Show in folder"}">Show</button>
        <button data-a="delete">Delete</button>
      </div>
    </div>
    <div class="meta"><b></b><span class="num"></span></div>`;
  const img = el.querySelector("img");
  img.src = r.thumb;
  // Stills by default; animate only the one under the pointer.
  el.addEventListener("mouseenter", () => (img.src = r.url));
  el.addEventListener("mouseleave", () => (img.src = r.thumb));
  el.querySelector("b").textContent = when(r.created);
  el.querySelector(".meta span").textContent = size(r.size);
  if (r.width) el.querySelector(".meta").title = `${r.width} × ${r.height} pixels`;

  const pic = el.querySelector(".pic");
  let startX = 0, startY = 0, isDown = false, dragged = false;

  pic.setAttribute("draggable", "true");
  pic.addEventListener("dragstart", (e) => {
    e.preventDefault();
    dragged = true;
    call("StartDrag", r.name);
  });

  pic.addEventListener("mousedown", (e) => {
    if (e.target.closest("button")) return;
    isDown = true;
    dragged = false;
    startX = e.clientX;
    startY = e.clientY;
  });

  window.addEventListener("mousemove", (e) => {
    if (!isDown || dragged) return;
    if (Math.hypot(e.clientX - startX, e.clientY - startY) > 5) {
      dragged = true;
      call("StartDrag", r.name);
    }
  });

  window.addEventListener("mouseup", () => {
    isDown = false;
  });

  pic.addEventListener("click", (e) => {
    if (e.target.closest("button") || dragged) return;
    copy(r.name);
  });
  pic.addEventListener("dblclick", () => call("Open", r.name));

  const del = el.querySelector('[data-a="delete"]');
  el.querySelector('[data-a="copy"]').onclick = () => copy(r.name);
  el.querySelector('[data-a="reveal"]').onclick = () => call("Reveal", r.name);
  del.onclick = async () => {
    if (!del.classList.contains("danger")) {
      del.classList.add("danger");
      del.textContent = "Delete?";
      setTimeout(() => { del.classList.remove("danger"); del.textContent = "Delete"; }, 2500);
      return;
    }
    await call("Delete", r.name);
    el.remove();
    $("empty").hidden = $("grid").children.length > 0;
  };
  el.addEventListener("mouseleave", () => { del.classList.remove("danger"); del.textContent = "Delete"; });
  if (fresh) {
    pic.animate([{ boxShadow: "0 0 0 3px var(--rec)" }, { boxShadow: "0 0 0 0 transparent" }], { duration: 1400, easing: "ease-out" });
  }
  return el;
}

async function copy(name) {
  try {
    await call("CopyFile", name);
    toast("Copied. Paste it anywhere.");
  } catch (e) {
    toast(String(e.message || e));
  }
}

// ---- settings ----

function renderSettings(st) {
  document.querySelectorAll(".seg").forEach((seg) => {
    const v = String(st[seg.dataset.key]);
    seg.querySelectorAll("button").forEach((b) => b.classList.toggle("on", b.dataset.v === v));
  });
  $("max").value = String(st.maxSeconds);
  $("countdown").checked = st.countdown;
  if ($("trim")) $("trim").checked = st.trim !== false;
  if ($("showCursor")) $("showCursor").checked = st.showCursor !== false;
  if ($("cursorHighlight")) $("cursorHighlight").checked = st.cursorHighlight !== false;
  if ($("clickRipples")) $("clickRipples").checked = st.clickRipples !== false;
  if ($("showControls")) $("showControls").checked = st.showControls !== false;
  $("folder").textContent = st.outputDir;
}

async function saveSettings(patch) {
  const next = { ...state.settings, ...patch };
  state.settings = await call("SaveSettings", next);
  renderSettings(state.settings);
}

document.querySelectorAll(".seg").forEach((seg) => {
  seg.addEventListener("click", (e) => {
    const b = e.target.closest("button");
    if (b) {
      const num = Number(b.dataset.v);
      const val = isNaN(num) ? b.dataset.v : num;
      saveSettings({ [seg.dataset.key]: val });
    }
  });
});
$("max").onchange = (e) => saveSettings({ maxSeconds: Number(e.target.value) });
$("countdown").onchange = (e) => saveSettings({ countdown: e.target.checked });
if ($("trim")) $("trim").onchange = (e) => saveSettings({ trim: e.target.checked });
if ($("showCursor")) $("showCursor").onchange = (e) => saveSettings({ showCursor: e.target.checked });
if ($("cursorHighlight")) $("cursorHighlight").onchange = (e) => saveSettings({ cursorHighlight: e.target.checked });
if ($("clickRipples")) $("clickRipples").onchange = (e) => saveSettings({ clickRipples: e.target.checked });
if ($("showControls")) $("showControls").onchange = (e) => saveSettings({ showControls: e.target.checked });
$("choose").onclick = async () => {
  await call("ChooseFolder");
  render(await call("GetState"));
};
$("open-settings").onclick = () => {
  document.querySelector(".pop")?.scrollTo(0, 0);
  $("sheet").classList.add("open");
  $("sheet").setAttribute("aria-hidden", "false");
};
$("close-settings").onclick = () => {
  $("sheet").classList.remove("open");
  $("sheet").setAttribute("aria-hidden", "true");
  document.querySelector(".pop")?.scrollTo(0, 0);
};

// ---- wiring ----

function startRecordingByMode(mode) {
  switch (mode) {
    case "window":
      call("StartWindow");
      break;
    case "region":
      call("StartRegion");
      break;
    case "screen":
      call("StartScreen");
      break;
    case "last":
      call("StartLast");
      break;
  }
}

document.querySelectorAll(".mode-card").forEach((card) => {
  card.addEventListener("click", () => {
    if (card.disabled) return;
    selectedMode = card.dataset.mode;
    updateModeUI();
  });
  card.addEventListener("dblclick", () => {
    if (card.disabled) return;
    startRecordingByMode(card.dataset.mode);
  });
});

const startBtn = $("start-btn");
if (startBtn) {
  startBtn.addEventListener("click", () => {
    startRecordingByMode(selectedMode);
  });
}

const openPerm = $("open-perm");
if (openPerm) {
  openPerm.addEventListener("click", () => {
    call("OpenPermissionSettings");
  });
}

const popoverPauseBtn = $("popover-pause-btn");
if (popoverPauseBtn) {
  popoverPauseBtn.addEventListener("click", () => {
    call("TogglePause");
  });
}

const trimStart = $("trim-start");
if (trimStart) {
  trimStart.addEventListener("input", (e) => {
    let val = Number(e.target.value);
    const endVal = Number($("trim-end")?.value || 0);
    if (val > endVal) {
      val = endVal;
      e.target.value = val;
    }
    stopPlayback();
    updateTimelineUI();
    showFrame(val);
  });
}

const trimEnd = $("trim-end");
if (trimEnd) {
  trimEnd.addEventListener("input", (e) => {
    let val = Number(e.target.value);
    const startVal = Number($("trim-start")?.value || 0);
    if (val < startVal) {
      val = startVal;
      e.target.value = val;
    }
    stopPlayback();
    updateTimelineUI();
    showFrame(val);
  });
}

const reviewPlayBtn = $("review-play-btn");
if (reviewPlayBtn) {
  reviewPlayBtn.addEventListener("click", () => {
    togglePlayback();
  });
}

const reviewSaveBtn = $("review-save-btn");
if (reviewSaveBtn) {
  reviewSaveBtn.addEventListener("click", async () => {
    stopPlayback();
    const startVal = Number($("trim-start")?.value || 0);
    const endVal = Number($("trim-end")?.value || 0);
    try {
      await call("ConfirmReview", startVal, endVal);
    } catch (err) {
      toast(String(err.message || err));
    }
  });
}

const reviewDiscardBtn = $("review-discard-btn");
if (reviewDiscardBtn) {
  reviewDiscardBtn.addEventListener("click", async () => {
    stopPlayback();
    await call("DiscardReview");
  });
}

document.querySelectorAll("[data-act]").forEach((b) => {
  b.addEventListener("click", () => call(b.dataset.act));
});

Events.On("state", (e) => render(e.data));
Events.On("review:open", (e) => setupReview(e.data));
Events.On("tick", (e) => {
  if (state?.phase === "recording" || state?.phase === "paused") {
    $("timer").textContent = clock(e.data);
  }
});
Events.On("countdown", (e) => { if (e.data > 0) $("timer").textContent = `Starting in ${e.data}`; });
Events.On("encode", (e) => setPct(e.data));
Events.On("library", (e) => loadLibrary(e.data));

// The popover is hidden rather than closed, so refresh whenever it's shown.
document.addEventListener("visibilitychange", async () => {
  if (!document.hidden) {
    document.querySelector(".pop")?.scrollTo(0, 0);
    try {
      render(await call("GetState"));
    } catch (e) {}
    try {
      await loadLibrary();
    } catch (e) {}
  }
});
window.addEventListener("focus", () => {
  document.querySelector(".pop")?.scrollTo(0, 0);
  loadLibrary();
});

async function init() {
  updateModeUI();
  document.querySelector(".pop")?.scrollTo(0, 0);
  try {
    const s = await call("GetState");
    render(s);
  } catch (err) {
    console.error("Failed to get state:", err);
  }
  try {
    await loadLibrary();
  } catch (err) {
    console.error("Failed to load library:", err);
  }
}
init();
