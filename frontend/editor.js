import { Call, Events } from "/wails/runtime.js";

const call = (method, ...args) => Call.ByName(`main.GifService.${method}`, ...args);
const $ = (id) => document.getElementById(id);

let reviewInfo = null;
let currentPlayhead = 0;
let isPlaying = false;
let playTimer = null;
let playbackSpeed = 1;
let exportFormat = "gif";
let maxColors = 256;
let activeCrop = null; // null or { x: 0..1, y: 0..1, w: 0..1, h: 0..1 }
let cropBoxState = { x: 0.1, y: 0.1, w: 0.8, h: 0.8 };
let isDraggingCrop = false;
let dragHandle = null;
let cropDragStart = { mouseX: 0, mouseY: 0, box: { ...cropBoxState } };
let annotations = [];
let activeTool = null;
let activeColor = "#ff3b30";
let isDrawing = false;
let currentAnnot = null;

let toastTimer;
function toast(msg) {
  const t = $("editor-toast");
  if (!t) return;
  t.textContent = msg;
  t.classList.add("show");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => t.classList.remove("show"), 1800);
}

// ---- Setup & Data Loading ----

async function initReview(info) {
  if (!info) {
    try {
      info = await call("GetReviewInfo");
    } catch (e) {
      return;
    }
  }
  if (!info || info.numFrames <= 0) return;
  reviewInfo = info;

  // Header meta badges
  const fileBadge = $("file-badge");
  const saveCopyBtn = $("btn-save-copy");
  if (info.fileName) {
    if (fileBadge) {
      fileBadge.hidden = false;
      fileBadge.textContent = info.fileName;
      fileBadge.title = info.fileName;
    }
    if (saveCopyBtn) saveCopyBtn.hidden = false;
    const ext = (info.fileName.split(".").pop() || "gif").toLowerCase();
    setFormat(ext);
  } else {
    if (fileBadge) fileBadge.hidden = true;
    if (saveCopyBtn) saveCopyBtn.hidden = true;
    setFormat("gif");
  }

  const fps = Math.max(1, info.fps || 15);
  if ($("meta-pill")) {
    $("meta-pill").textContent = `${fps} FPS • ${info.numFrames} frames`;
  }

  // Setup dual trim sliders
  const startSlider = $("trim-start-slider");
  const endSlider = $("trim-end-slider");
  if (startSlider && endSlider) {
    startSlider.min = 0;
    startSlider.max = info.numFrames - 1;
    startSlider.value = 0;

    endSlider.min = 0;
    endSlider.max = info.numFrames - 1;
    endSlider.value = info.numFrames - 1;
  }

  currentPlayhead = 0;
  annotations = [];
  undoStack = [];
  redoStack = [];
  selectedAnnotIndex = null;
  updateHistoryButtons();
  setActiveTool(null);
  stopPlayback();
  updateTimeline();
  showFrame(0);
}

// ---- Frame Display & Playback ----

function showFrame(idx) {
  if (!reviewInfo) return;
  idx = Math.max(0, Math.min(idx, (reviewInfo.numFrames || 1) - 1));
  currentPlayhead = idx;

  const img = $("preview-img");
  if (img) {
    img.src = `/preview/frame?i=${idx}&t=${Date.now()}`;
    img.onload = () => resizeCanvas();
  }

  const pill = $("frame-indicator-pill");
  if (pill) {
    pill.textContent = `Frame ${idx + 1} / ${reviewInfo.numFrames}`;
  }

  const fps = Math.max(1, reviewInfo.fps || 15);
  const curSec = (idx / fps).toFixed(2);
  const headInfo = $("playhead-info");
  if (headInfo) {
    headInfo.textContent = `Playhead: Frame ${idx + 1} (${curSec}s)`;
  }

  updateNeedle();
}

function updateTimeline() {
  if (!reviewInfo) return;
  const total = Math.max(1, reviewInfo.numFrames - 1);
  const startSlider = $("trim-start-slider");
  const endSlider = $("trim-end-slider");
  if (!startSlider || !endSlider) return;

  const sVal = Number(startSlider.value);
  const eVal = Number(endSlider.value);

  const sPct = (sVal / total) * 100;
  const ePct = (eVal / total) * 100;

  const hl = $("timeline-range-highlight");
  if (hl) {
    hl.style.left = `${sPct}%`;
    hl.style.width = `${Math.max(0, ePct - sPct)}%`;
  }

  const fps = Math.max(1, reviewInfo.fps || 15);
  const sSec = (sVal / fps).toFixed(1);
  const eSec = (eVal / fps).toFixed(1);
  const count = eVal - sVal + 1;
  const durSec = (count / fps).toFixed(1);
  const totalSec = reviewInfo.duration ? reviewInfo.duration.toFixed(1) : (reviewInfo.numFrames / fps).toFixed(1);

  if ($("trim-range-badge")) {
    $("trim-range-badge").textContent = `${sSec}s – ${eSec}s (${durSec}s)`;
  }
  if ($("trim-frames-count")) {
    $("trim-frames-count").textContent = `(${count} of ${reviewInfo.numFrames} frames)`;
  }
  if ($("time-readout")) {
    $("time-readout").textContent = `${durSec}s / ${totalSec}s`;
  }

  updateNeedle();
}

function updateNeedle() {
  if (!reviewInfo) return;
  const total = Math.max(1, reviewInfo.numFrames - 1);
  const pct = (currentPlayhead / total) * 100;
  const needle = $("timeline-playhead-needle");
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

  const sVal = Number($("trim-start-slider")?.value || 0);
  const eVal = Number($("trim-end-slider")?.value || 0);
  if (currentPlayhead < sVal || currentPlayhead >= eVal) {
    currentPlayhead = sVal;
  }

  const fps = Math.max(1, reviewInfo.fps || 15);
  const interval = Math.max(16, Math.round(1000 / (fps * playbackSpeed)));

  clearInterval(playTimer);
  playTimer = setInterval(() => {
    const s = Number($("trim-start-slider")?.value || 0);
    const e = Number($("trim-end-slider")?.value || 0);
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

function setPlaybackSpeed(spd) {
  playbackSpeed = spd;
  document.querySelectorAll("#speed-selector button").forEach((b) => {
    b.classList.toggle("on", Number(b.dataset.spd) === spd);
  });
  if (isPlaying) {
    stopPlayback();
    startPlayback();
  }
}

// ---- Canvas & Annotations ----

const canvas = $("annot-canvas");
const ctx = canvas ? canvas.getContext("2d") : null;

function resizeCanvas() {
  const img = $("preview-img");
  if (!canvas || !img) return;
  const rect = img.getBoundingClientRect();
  if (rect.width > 0 && rect.height > 0) {
    canvas.width = rect.width;
    canvas.height = rect.height;
    renderAnnotations();
  }
}

window.addEventListener("resize", () => resizeCanvas());

function setFormat(fmt) {
  exportFormat = fmt || "gif";
  document.querySelectorAll("#editor-format-toggle button").forEach((b) => {
    b.classList.toggle("on", b.dataset.fmt === exportFormat);
  });
  const palWrap = $("palette-choice-wrap");
  if (palWrap) {
    palWrap.style.display = exportFormat === "gif" ? "flex" : "none";
  }
  const txt = $("btn-save-text");
  if (txt) {
    if (reviewInfo?.fileName) {
      txt.textContent = `Save (${exportFormat.toUpperCase()})`;
    } else {
      txt.textContent = `Save ${exportFormat.toUpperCase()}`;
    }
  }
}

document.querySelectorAll("#editor-format-toggle button").forEach((b) => {
  b.addEventListener("click", () => setFormat(b.dataset.fmt));
});

function setActiveTool(tool) {
  activeTool = activeTool === tool ? null : tool;
  selectedAnnotIndex = null;
  updateHistoryButtons();
  hideInlineTextBox();

  document.querySelectorAll("#annot-tools button").forEach((b) => {
    b.classList.toggle("on", b.dataset.tool === activeTool || (b.dataset.tool === "crop" && activeCrop !== null));
  });
  if (canvas) {
    canvas.classList.toggle("drawing", Boolean(activeTool && activeTool !== "crop" && activeTool !== "text"));
    canvas.classList.toggle("text-mode", activeTool === "text");
  }
  const cropOverlay = $("crop-overlay");
  if (cropOverlay) {
    cropOverlay.hidden = activeTool !== "crop";
    if (activeTool === "crop") {
      updateCropBoxDOM();
    }
  }
  renderAnnotations();
}

function updateCropBoxDOM() {
  const box = $("crop-box");
  const img = $("preview-img");
  if (!box) return;

  box.style.left = `${(cropBoxState.x * 100).toFixed(2)}%`;
  box.style.top = `${(cropBoxState.y * 100).toFixed(2)}%`;
  box.style.width = `${(cropBoxState.w * 100).toFixed(2)}%`;
  box.style.height = `${(cropBoxState.h * 100).toFixed(2)}%`;

  const natW = img?.naturalWidth || 800;
  const natH = img?.naturalHeight || 600;
  const pxW = Math.round(cropBoxState.w * natW);
  const pxH = Math.round(cropBoxState.h * natH);
  const dims = $("crop-dims");
  if (dims) {
    dims.textContent = `${pxW} × ${pxH} px`;
  }
}

// Crop box dragging and resizing
const cropBox = $("crop-box");
const cropOverlay = $("crop-overlay");

if (cropBox && cropOverlay) {
  cropBox.addEventListener("mousedown", (e) => {
    const handle = e.target.closest(".crop-handle");
    const toolbar = e.target.closest(".crop-toolbar");
    if (toolbar) return;

    e.preventDefault();
    e.stopPropagation();

    isDraggingCrop = true;
    dragHandle = handle ? handle.dataset.handle : null;
    cropDragStart = {
      mouseX: e.clientX,
      mouseY: e.clientY,
      box: { ...cropBoxState },
    };
  });

  window.addEventListener("mousemove", (e) => {
    if (!isDraggingCrop || !cropOverlay) return;
    const rect = cropOverlay.getBoundingClientRect();
    if (!rect.width || !rect.height) return;

    const dx = (e.clientX - cropDragStart.mouseX) / rect.width;
    const dy = (e.clientY - cropDragStart.mouseY) / rect.height;
    const initial = cropDragStart.box;

    if (!dragHandle) {
      const newX = Math.max(0, Math.min(1 - initial.w, initial.x + dx));
      const newY = Math.max(0, Math.min(1 - initial.h, initial.y + dy));
      cropBoxState.x = newX;
      cropBoxState.y = newY;
    } else {
      if (dragHandle === "se") {
        cropBoxState.w = Math.max(0.05, Math.min(1 - initial.x, initial.w + dx));
        cropBoxState.h = Math.max(0.05, Math.min(1 - initial.y, initial.h + dy));
      } else if (dragHandle === "sw") {
        const newW = Math.max(0.05, initial.w - dx);
        const newX = initial.x + (initial.w - newW);
        if (newX >= 0) {
          cropBoxState.x = newX;
          cropBoxState.w = newW;
        }
        cropBoxState.h = Math.max(0.05, Math.min(1 - initial.y, initial.h + dy));
      } else if (dragHandle === "ne") {
        cropBoxState.w = Math.max(0.05, Math.min(1 - initial.x, initial.w + dx));
        const newH = Math.max(0.05, initial.h - dy);
        const newY = initial.y + (initial.h - newH);
        if (newY >= 0) {
          cropBoxState.y = newY;
          cropBoxState.h = newH;
        }
      } else if (dragHandle === "nw") {
        const newW = Math.max(0.05, initial.w - dx);
        const newX = initial.x + (initial.w - newW);
        const newH = Math.max(0.05, initial.h - dy);
        const newY = initial.y + (initial.h - newH);
        if (newX >= 0) {
          cropBoxState.x = newX;
          cropBoxState.w = newW;
        }
        if (newY >= 0) {
          cropBoxState.y = newY;
          cropBoxState.h = newH;
        }
      }
    }
    updateCropBoxDOM();
  });

  window.addEventListener("mouseup", () => {
    if (isDraggingCrop) {
      isDraggingCrop = false;
      dragHandle = null;
    }
  });
}

$("btn-crop-apply")?.addEventListener("click", (e) => {
  e.stopPropagation();
  activeCrop = { ...cropBoxState };
  const img = $("preview-img");
  const natW = img?.naturalWidth || 800;
  const natH = img?.naturalHeight || 600;
  const pxW = Math.round(activeCrop.w * natW);
  const pxH = Math.round(activeCrop.h * natH);
  toast(`Crop applied: ${pxW} × ${pxH} px`);
  setActiveTool(null);
});

$("btn-crop-reset")?.addEventListener("click", (e) => {
  e.stopPropagation();
  activeCrop = null;
  cropBoxState = { x: 0.1, y: 0.1, w: 0.8, h: 0.8 };
  updateCropBoxDOM();
  setActiveTool(null);
  toast("Crop reset to full frame");
});

// Palette toggle listeners
document.querySelectorAll("#editor-palette-toggle button").forEach((b) => {
  b.addEventListener("click", () => {
    maxColors = Number(b.dataset.colors) || 256;
    document.querySelectorAll("#editor-palette-toggle button").forEach((btn) => {
      btn.classList.toggle("on", btn === b);
    });
  });
});

// ---- History & Annotations State ----
let undoStack = [];
let redoStack = [];
let selectedAnnotIndex = null;
let pendingTextPos = null;

function updateHistoryButtons() {
  const undoBtn = $("btn-undo");
  const redoBtn = $("btn-redo");
  const delBtn = $("btn-delete-annot");
  if (undoBtn) undoBtn.disabled = undoStack.length === 0;
  if (redoBtn) redoBtn.disabled = redoStack.length === 0;
  if (delBtn) delBtn.disabled = selectedAnnotIndex === null;
}

function pushHistory() {
  undoStack.push(JSON.parse(JSON.stringify(annotations)));
  if (undoStack.length > 50) undoStack.shift();
  redoStack = [];
  updateHistoryButtons();
}

function undo() {
  if (undoStack.length === 0) return;
  hideInlineTextBox();
  redoStack.push(JSON.parse(JSON.stringify(annotations)));
  annotations = undoStack.pop() || [];
  selectedAnnotIndex = null;
  updateHistoryButtons();
  renderAnnotations();
  toast("Undo");
}

function redo() {
  if (redoStack.length === 0) return;
  hideInlineTextBox();
  undoStack.push(JSON.parse(JSON.stringify(annotations)));
  annotations = redoStack.pop() || [];
  selectedAnnotIndex = null;
  updateHistoryButtons();
  renderAnnotations();
  toast("Redo");
}

function deleteSelected() {
  if (selectedAnnotIndex !== null && selectedAnnotIndex >= 0 && selectedAnnotIndex < annotations.length) {
    pushHistory();
    annotations.splice(selectedAnnotIndex, 1);
    selectedAnnotIndex = null;
    updateHistoryButtons();
    renderAnnotations();
    toast("Item removed");
  }
}

function clearAllAnnotations() {
  if (annotations.length === 0) return;
  pushHistory();
  annotations = [];
  selectedAnnotIndex = null;
  hideInlineTextBox();
  updateHistoryButtons();
  renderAnnotations();
  toast("All items cleared");
}

$("btn-undo")?.addEventListener("click", undo);
$("btn-redo")?.addEventListener("click", redo);
$("btn-delete-annot")?.addEventListener("click", deleteSelected);
$("clear-annot")?.addEventListener("click", clearAllAnnotations);

document.querySelectorAll("#annot-tools button").forEach((b) => {
  b.addEventListener("click", () => setActiveTool(b.dataset.tool));
});

document.querySelectorAll("#color-picker button").forEach((b) => {
  b.addEventListener("click", () => {
    activeColor = b.dataset.col;
    document.querySelectorAll("#color-picker button").forEach((el) => el.classList.remove("on"));
    b.classList.add("on");

    const input = $("inline-text-input");
    if (input) input.style.color = activeColor;

    if (selectedAnnotIndex !== null && annotations[selectedAnnotIndex]) {
      pushHistory();
      annotations[selectedAnnotIndex].color = activeColor;
      renderAnnotations();
    }
  });
});

// Inline Text Box Management
function showInlineTextBox(canvasX, canvasY, clientX, clientY, initialText = "") {
  const box = $("inline-text-box");
  const input = $("inline-text-input");
  const viewport = $("viewport-box");
  if (!box || !input || !viewport) return;

  const vpRect = viewport.getBoundingClientRect();
  const relX = clientX - vpRect.left;
  const relY = clientY - vpRect.top;

  pendingTextPos = { canvasX, canvasY };

  box.style.left = `${Math.max(10, Math.min(vpRect.width - 240, relX))}px`;
  box.style.top = `${Math.max(20, Math.min(vpRect.height - 35, relY))}px`;
  box.hidden = false;
  box.style.display = "flex";

  input.value = initialText;
  input.style.color = activeColor;
  setTimeout(() => {
    input.focus();
    if (initialText) input.select();
  }, 10);
}

function hideInlineTextBox() {
  const box = $("inline-text-box");
  if (!box) return;
  box.hidden = true;
  box.style.display = "none";
  pendingTextPos = null;
}

function commitInlineText() {
  const input = $("inline-text-input");
  if (!input || !pendingTextPos) {
    hideInlineTextBox();
    return;
  }
  const txt = input.value.trim();
  if (txt) {
    pushHistory();
    annotations.push({
      type: "text",
      x: pendingTextPos.canvasX,
      y: pendingTextPos.canvasY,
      text: txt,
      color: activeColor,
    });
    selectedAnnotIndex = annotations.length - 1;
    updateHistoryButtons();
    renderAnnotations();
  }
  hideInlineTextBox();
}

$("inline-text-commit")?.addEventListener("click", commitInlineText);
$("inline-text-cancel")?.addEventListener("click", hideInlineTextBox);
$("inline-text-input")?.addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    commitInlineText();
  } else if (e.key === "Escape") {
    e.preventDefault();
    hideInlineTextBox();
  }
});

function drawArrow(context, fromX, fromY, toX, toY, color) {
  const headLen = 14;
  const angle = Math.atan2(toY - fromY, toX - fromX);
  context.strokeStyle = color;
  context.fillStyle = color;
  context.lineWidth = 3.5;
  context.lineCap = "round";

  context.beginPath();
  context.moveTo(fromX, fromY);
  context.lineTo(toX, toY);
  context.stroke();

  context.beginPath();
  context.moveTo(toX, toY);
  context.lineTo(toX - headLen * Math.cos(angle - Math.PI / 6), toY - headLen * Math.sin(angle - Math.PI / 6));
  context.lineTo(toX - headLen * Math.cos(angle + Math.PI / 6), toY - headLen * Math.sin(angle + Math.PI / 6));
  context.closePath();
  context.fill();
}

function distToSegment(px, py, x1, y1, x2, y2) {
  const dx = x2 - x1;
  const dy = y2 - y1;
  const lenSq = dx * dx + dy * dy;
  if (lenSq === 0) return Math.hypot(px - x1, py - y1);
  const t = Math.max(0, Math.min(1, ((px - x1) * dx + (py - y1) * dy) / lenSq));
  const projX = x1 + t * dx;
  const projY = y1 + t * dy;
  return Math.hypot(px - projX, py - projY);
}

function hitTestAnnotation(px, py, w, h) {
  if (!ctx) return -1;
  for (let i = annotations.length - 1; i >= 0; i--) {
    const a = annotations[i];
    if (a.type === "arrow") {
      const ax1 = (a.x ?? a.x1 ?? 0) * w;
      const ay1 = (a.y ?? a.y1 ?? 0) * h;
      const ax2 = (a.x2 ?? 0) * w;
      const ay2 = (a.y2 ?? 0) * h;
      if (distToSegment(px, py, ax1, ay1, ax2, ay2) <= 12) {
        return i;
      }
    } else if (a.type === "blur") {
      const bx = a.x * w;
      const by = a.y * h;
      const bw = a.w * w;
      const bh = a.h * h;
      if (px >= bx && px <= bx + bw && py >= by && py <= by + bh) {
        return i;
      }
    } else if (a.type === "text") {
      const tx = a.x * w;
      const ty = a.y * h;
      ctx.font = "bold 15px -apple-system, BlinkMacSystemFont, sans-serif";
      const tw = ctx.measureText(a.text).width;
      if (px >= tx - 10 && px <= tx + tw + 10 && py >= ty - 22 && py <= ty + 8) {
        return i;
      }
    }
  }
  return -1;
}

function renderAnnotations() {
  if (!ctx || !canvas) return;
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  const w = canvas.width;
  const h = canvas.height;

  const all = currentAnnot ? [...annotations, currentAnnot] : annotations;
  for (let i = 0; i < all.length; i++) {
    const a = all[i];
    const isSelected = i === selectedAnnotIndex && !currentAnnot;

    if (a.type === "arrow") {
      const fromX = (a.x ?? a.x1 ?? 0) * w;
      const fromY = (a.y ?? a.y1 ?? 0) * h;
      const toX = (a.x2 ?? 0) * w;
      const toY = (a.y2 ?? 0) * h;
      drawArrow(ctx, fromX, fromY, toX, toY, a.color);

      if (isSelected) {
        ctx.strokeStyle = "#00d2d3";
        ctx.lineWidth = 1.5;
        ctx.setLineDash([3, 3]);
        ctx.beginPath();
        ctx.arc(fromX, fromY, 6, 0, Math.PI * 2);
        ctx.arc(toX, toY, 6, 0, Math.PI * 2);
        ctx.stroke();
        ctx.setLineDash([]);
      }
    } else if (a.type === "blur") {
      const rx = a.x * w;
      const ry = a.y * h;
      const rw = a.w * w;
      const rh = a.h * h;
      ctx.fillStyle = "rgba(0, 0, 0, 0.72)";
      ctx.fillRect(rx, ry, rw, rh);
      ctx.strokeStyle = isSelected ? "#00d2d3" : "rgba(255, 255, 255, 0.4)";
      ctx.lineWidth = isSelected ? 2 : 1.5;
      if (isSelected) ctx.setLineDash([4, 4]);
      ctx.strokeRect(rx, ry, rw, rh);
      if (isSelected) ctx.setLineDash([]);

      // Hatching stripes
      ctx.save();
      ctx.beginPath();
      ctx.rect(rx, ry, rw, rh);
      ctx.clip();
      ctx.strokeStyle = "rgba(255, 255, 255, 0.15)";
      ctx.lineWidth = 1;
      for (let x = rx - rh; x < rx + rw; x += 10) {
        ctx.beginPath();
        ctx.moveTo(x, ry);
        ctx.lineTo(x + rh, ry + rh);
        ctx.stroke();
      }
      ctx.restore();
    } else if (a.type === "text") {
      ctx.font = "bold 15px -apple-system, BlinkMacSystemFont, sans-serif";
      const tx = a.x * w;
      const ty = a.y * h;
      const metrics = ctx.measureText(a.text);
      const textW = metrics.width;
      const padX = 8;
      const padY = 4;
      const boxH = 20;

      // Draw dark background pill
      ctx.fillStyle = "rgba(18, 20, 26, 0.88)";
      ctx.beginPath();
      ctx.roundRect(tx - padX, ty - boxH + padY, textW + padX * 2, boxH + padY, 4);
      ctx.fill();

      if (isSelected) {
        ctx.strokeStyle = "#00d2d3";
        ctx.lineWidth = 1.5;
        ctx.setLineDash([3, 3]);
        ctx.stroke();
        ctx.setLineDash([]);
      }

      ctx.fillStyle = a.color || "#ffffff";
      ctx.fillText(a.text, tx, ty);
    }
  }
}

if (canvas) {
  canvas.addEventListener("mousedown", (e) => {
    const rect = canvas.getBoundingClientRect();
    const x = (e.clientX - rect.left) / rect.width;
    const y = (e.clientY - rect.top) / rect.height;

    if (activeTool === "text") {
      showInlineTextBox(x, y, e.clientX, e.clientY);
      return;
    }

    if (!activeTool) {
      const hit = hitTestAnnotation(e.clientX - rect.left, e.clientY - rect.top, rect.width, rect.height);
      if (hit >= 0) {
        selectedAnnotIndex = hit;
      } else {
        selectedAnnotIndex = null;
        hideInlineTextBox();
      }
      updateHistoryButtons();
      renderAnnotations();
      return;
    }

    hideInlineTextBox();
    selectedAnnotIndex = null;
    updateHistoryButtons();
    isDrawing = true;

    if (activeTool === "arrow") {
      currentAnnot = { type: "arrow", x: x, y: y, x1: x, y1: y, x2: x, y2: y, color: activeColor };
    } else if (activeTool === "blur") {
      currentAnnot = { type: "blur", x0: x, y0: y, x: x, y: y, w: 0, h: 0 };
    }
  });

  window.addEventListener("mousemove", (e) => {
    if (!isDrawing || !currentAnnot || !canvas) return;
    const rect = canvas.getBoundingClientRect();
    const x = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
    const y = Math.max(0, Math.min(1, (e.clientY - rect.top) / rect.height));

    if (currentAnnot.type === "arrow") {
      currentAnnot.x2 = x;
      currentAnnot.y2 = y;
    } else if (currentAnnot.type === "blur") {
      currentAnnot.x = Math.min(currentAnnot.x0, x);
      currentAnnot.y = Math.min(currentAnnot.y0, y);
      currentAnnot.w = Math.abs(x - currentAnnot.x0);
      currentAnnot.h = Math.abs(y - currentAnnot.y0);
    }
    renderAnnotations();
  });

  window.addEventListener("mouseup", () => {
    if (isDrawing && currentAnnot) {
      isDrawing = false;
      delete currentAnnot.x0;
      delete currentAnnot.y0;

      let valid = true;
      if (currentAnnot.type === "arrow") {
        valid = Math.hypot(currentAnnot.x2 - currentAnnot.x, currentAnnot.y2 - currentAnnot.y) > 0.01;
      } else if (currentAnnot.type === "blur") {
        valid = currentAnnot.w > 0.01 && currentAnnot.h > 0.01;
      }

      if (valid) {
        pushHistory();
        annotations.push(currentAnnot);
        selectedAnnotIndex = annotations.length - 1;
        updateHistoryButtons();
      }
      currentAnnot = null;
      renderAnnotations();
    }
  });
}

// ---- Sliders & Timeline Wiring ----

const startSlider = $("trim-start-slider");
if (startSlider) {
  startSlider.addEventListener("input", (e) => {
    let val = Number(e.target.value);
    const endVal = Number($("trim-end-slider")?.value || 0);
    if (val > endVal) {
      val = endVal;
      e.target.value = val;
    }
    stopPlayback();
    updateTimeline();
    showFrame(val);
  });
}

const endSlider = $("trim-end-slider");
if (endSlider) {
  endSlider.addEventListener("input", (e) => {
    let val = Number(e.target.value);
    const startVal = Number($("trim-start-slider")?.value || 0);
    if (val < startVal) {
      val = startVal;
      e.target.value = val;
    }
    stopPlayback();
    updateTimeline();
    showFrame(val);
  });
}

const track = $("timeline-track");
if (track) {
  track.addEventListener("click", (e) => {
    if (!reviewInfo || reviewInfo.numFrames <= 0) return;
    const b = track.getBoundingClientRect();
    const pct = Math.max(0, Math.min(1, (e.clientX - b.left) / b.width));
    const targetIdx = Math.round(pct * (reviewInfo.numFrames - 1));
    stopPlayback();
    showFrame(targetIdx);
  });
}

// ---- Buttons & Shortcuts ----

$("play-btn")?.addEventListener("click", togglePlayback);
$("step-back")?.addEventListener("click", () => {
  stopPlayback();
  showFrame(currentPlayhead - 1);
});
$("step-fwd")?.addEventListener("click", () => {
  stopPlayback();
  showFrame(currentPlayhead + 1);
});

document.querySelectorAll("#speed-selector button").forEach((b) => {
  b.addEventListener("click", () => setPlaybackSpeed(Number(b.dataset.spd)));
});

$("btn-cancel")?.addEventListener("click", async () => {
  stopPlayback();
  await call("DiscardReview");
});

function setEncodingOverlay(show) {
  const overlay = $("encoding-overlay");
  if (!overlay) return;
  if (show) {
    overlay.hidden = false;
    overlay.style.display = "flex";
  } else {
    overlay.hidden = true;
    overlay.style.display = "none";
  }
}

async function saveReview(saveAsCopy) {
  stopPlayback();
  const startVal = Number($("trim-start-slider")?.value || 0);
  const endVal = Number($("trim-end-slider")?.value || 0);

  const pctEl = $("encoding-pct");
  const barEl = $("encoding-bar-fill");
  if (pctEl) pctEl.textContent = "0%";
  if (barEl) barEl.style.width = "0%";
  setEncodingOverlay(true);

  try {
    await call("ConfirmReview", startVal, endVal, exportFormat, annotations, saveAsCopy, activeCrop, maxColors);
  } catch (err) {
    setEncodingOverlay(false);
    toast(String(err.message || err));
  }
}

$("btn-save")?.addEventListener("click", () => saveReview(false));
$("btn-save-copy")?.addEventListener("click", () => saveReview(true));

// Keyboard shortcuts
window.addEventListener("keydown", (e) => {
  const isInput = e.target && (e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA");
  if (isInput) {
    if (e.target.id === "inline-text-input") {
      // Let inline text input listener handle Enter / Escape
      return;
    }
    return;
  }

  const isCmdOrCtrl = e.metaKey || e.ctrlKey;
  if (isCmdOrCtrl && e.key.toLowerCase() === "z") {
    e.preventDefault();
    if (e.shiftKey) {
      redo();
    } else {
      undo();
    }
    return;
  }
  if (isCmdOrCtrl && e.key.toLowerCase() === "y") {
    e.preventDefault();
    redo();
    return;
  }
  if (e.key === "Backspace" || e.key === "Delete") {
    if (selectedAnnotIndex !== null) {
      e.preventDefault();
      deleteSelected();
      return;
    }
  }

  if (e.code === "Space") {
    e.preventDefault();
    togglePlayback();
  } else if (e.key === "ArrowLeft") {
    e.preventDefault();
    stopPlayback();
    showFrame(currentPlayhead - 1);
  } else if (e.key === "ArrowRight") {
    e.preventDefault();
    stopPlayback();
    showFrame(currentPlayhead + 1);
  } else if (e.key === "Escape") {
    e.preventDefault();
    if ($("inline-text-box") && !$("inline-text-box").hidden) {
      hideInlineTextBox();
    } else if (selectedAnnotIndex !== null) {
      selectedAnnotIndex = null;
      updateHistoryButtons();
      renderAnnotations();
    } else if (activeTool) {
      setActiveTool(null);
    } else {
      $("btn-cancel")?.click();
    }
  } else if (e.key === "Enter") {
    e.preventDefault();
    $("btn-save")?.click();
  }
});

// Wails Event Listeners
Events.On("review:open", (e) => {
  setEncodingOverlay(false);
  initReview(e.data);
});
Events.On("encode", (e) => {
  const p = e.data;
  const pctEl = $("encoding-pct");
  const barEl = $("encoding-bar-fill");
  if (pctEl) pctEl.textContent = `${p}%`;
  if (barEl) barEl.style.width = `${p}%`;
});
Events.On("library", () => {
  setEncodingOverlay(false);
  call("CloseEditor").catch(() => {});
});
Events.On("encode:error", (e) => {
  setEncodingOverlay(false);
  toast("Encoding failed: " + (e?.data || e || "unknown error"));
});
Events.On("state", (e) => {
  const s = e?.data;
  if (s && s.phase === "idle") {
    setEncodingOverlay(false);
    if (s.error) toast("Error: " + s.error);
  }
});

// Initial boot check
initReview();
