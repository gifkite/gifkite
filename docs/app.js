// Gifkite Interactive Documentation & Playground Logic

document.addEventListener('DOMContentLoaded', () => {
  initCopyButtons();
  initViewfinderDemo();
  initDitheringDemo();
  initRippleSandbox();
  initTrimTimeline();
  initTabs();
});

// 1. Copy Buttons
function initCopyButtons() {
  document.querySelectorAll('[data-copy]').forEach(btn => {
    btn.addEventListener('click', async () => {
      const text = btn.getAttribute('data-copy');
      try {
        await navigator.clipboard.writeText(text);
        const original = btn.innerHTML;
        btn.innerHTML = '✓ Copied';
        setTimeout(() => { btn.innerHTML = original; }, 1800);
      } catch (e) {
        console.error('Clipboard copy failed:', e);
      }
    });
  });
}

// 2. Tab Navigation for Interactive Demos
function initTabs() {
  const tabs = document.querySelectorAll('.tab-btn');
  const views = document.querySelectorAll('.demo-view');

  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => t.classList.remove('active'));
      views.forEach(v => v.classList.remove('active'));

      tab.classList.add('active');
      const targetId = tab.getAttribute('data-target');
      const targetView = document.getElementById(targetId);
      if (targetView) targetView.classList.add('active');

      if (targetId === 'demo-dither') {
        renderDitherDemo();
      }
    });
  });
}

// 3. Viewfinder Live Simulator
function initViewfinderDemo() {
  const container = document.getElementById('vf-container');
  const box = document.getElementById('vf-box');
  const dimText = document.getElementById('vf-dim');
  const recTime = document.getElementById('vf-rectime');
  const recDot = document.getElementById('vf-recdot');
  const btnToggle = document.getElementById('vf-toggle-rec');

  if (!container || !box) return;

  let isRecording = true;
  let seconds = 3.4;
  let timer = setInterval(() => {
    if (isRecording) {
      seconds += 0.1;
      if (recTime) recTime.textContent = `00:0${seconds.toFixed(1)}s`;
    }
  }, 100);

  if (btnToggle) {
    btnToggle.addEventListener('click', () => {
      isRecording = !isRecording;
      btnToggle.textContent = isRecording ? 'Pause' : 'Resume';
      if (recDot) recDot.style.opacity = isRecording ? '1' : '0.3';
    });
  }

  // Dragging logic within viewfinder container
  let isDragging = false;
  let startX, startY, initLeft, initTop;

  box.addEventListener('mousedown', (e) => {
    if (e.target.closest('button')) return;
    isDragging = true;
    startX = e.clientX;
    startY = e.clientY;
    initLeft = box.offsetLeft;
    initTop = box.offsetTop;
    box.style.transition = 'none';
  });

  window.addEventListener('mousemove', (e) => {
    if (!isDragging) return;
    const dx = e.clientX - startX;
    const dy = e.clientY - startY;

    const maxLeft = container.clientWidth - box.offsetWidth;
    const maxTop = container.clientHeight - box.offsetHeight;

    const newLeft = Math.max(0, Math.min(maxLeft, initLeft + dx));
    const newTop = Math.max(0, Math.min(maxTop, initTop + dy));

    box.style.left = `${newLeft}px`;
    box.style.top = `${newTop}px`;
    box.style.position = 'absolute';

    if (dimText) {
      dimText.textContent = `${Math.round(box.offsetWidth * 2)} × ${Math.round(box.offsetHeight * 2)} px`;
    }
  });

  window.addEventListener('mouseup', () => {
    isDragging = false;
  });
}

// 4. Dithering Interactive Simulator
let currentDither = 'bayer';
function initDitheringDemo() {
  const buttons = document.querySelectorAll('.dither-btn');
  buttons.forEach(btn => {
    btn.addEventListener('click', () => {
      buttons.forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      currentDither = btn.getAttribute('data-dither');
      renderDitherDemo();
    });
  });

  renderDitherDemo();
}

function renderDitherDemo() {
  const canvas = document.getElementById('dither-canvas');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  const w = canvas.width = 600;
  const h = canvas.height = 240;

  // Draw rich test gradient
  const grad = ctx.createLinearGradient(0, 0, w, h);
  grad.addColorStop(0, '#0f172a');
  grad.addColorStop(0.3, '#ff3b5c');
  grad.addColorStop(0.65, '#ff9f43');
  grad.addColorStop(1, '#00d2d3');

  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, w, h);

  const imgData = ctx.getImageData(0, 0, w, h);
  const data = imgData.data;

  // Bayer 4x4 matrix normalized
  const bayer4 = [
    [ 0,  8,  2, 10],
    [12,  4, 14,  6],
    [ 3, 11,  1,  9],
    [15,  7, 13,  5]
  ];

  const badge = document.getElementById('dither-metric');

  if (currentDither === 'bayer') {
    for (let y = 0; y < h; y++) {
      for (let x = 0; x < w; x++) {
        const idx = (y * w + x) * 4;
        const threshold = (bayer4[y % 4][x % 4] / 16 - 0.5) * 48;
        data[idx]     = Math.min(255, Math.max(0, quantizeStep(data[idx] + threshold, 32)));
        data[idx + 1] = Math.min(255, Math.max(0, quantizeStep(data[idx + 1] + threshold, 32)));
        data[idx + 2] = Math.min(255, Math.max(0, quantizeStep(data[idx + 2] + threshold, 32)));
      }
    }
    if (badge) badge.textContent = 'Size: ~180 KB • Smooth Gradients • Smallest File';
  } else if (currentDither === 'floyd') {
    // Simulated error diffusion appearance
    for (let y = 0; y < h; y++) {
      for (let x = 0; x < w; x++) {
        const idx = (y * w + x) * 4;
        const noise = ((Math.random() - 0.5) * 30);
        data[idx]     = quantizeStep(data[idx] + noise, 40);
        data[idx + 1] = quantizeStep(data[idx + 1] + noise, 40);
        data[idx + 2] = quantizeStep(data[idx + 2] + noise, 40);
      }
    }
    if (badge) badge.textContent = 'Size: ~420 KB • Maximum Color Fidelity • Larger File';
  } else {
    // None
    for (let i = 0; i < data.length; i += 4) {
      data[i]     = quantizeStep(data[i], 64);
      data[i + 1] = quantizeStep(data[i + 1], 64);
      data[i + 2] = quantizeStep(data[i + 2], 64);
    }
    if (badge) badge.textContent = 'Size: ~95 KB • Posterized Color Banding';
  }

  ctx.putImageData(imgData, 0, 0);
}

function quantizeStep(val, step) {
  return Math.round(val / step) * step;
}

// 5. Cursor Spotlight & Click Ripple Sandbox
function initRippleSandbox() {
  const canvas = document.getElementById('ripple-canvas');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');

  function resize() {
    canvas.width = canvas.parentElement.clientWidth;
    canvas.height = 320;
  }
  resize();
  window.addEventListener('resize', resize);

  let mouse = { x: canvas.width / 2, y: canvas.height / 2, active: false };
  let ripples = [];

  canvas.addEventListener('mousemove', (e) => {
    const rect = canvas.getBoundingClientRect();
    mouse.x = e.clientX - rect.left;
    mouse.y = e.clientY - rect.top;
    mouse.active = true;
  });

  canvas.addEventListener('mouseleave', () => {
    mouse.active = false;
  });

  canvas.addEventListener('mousedown', (e) => {
    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    const isRight = e.button === 2;

    ripples.push({
      x, y,
      radius: 4,
      maxRadius: 36,
      opacity: 0.9,
      color: isRight ? '#ff9f43' : '#ff3b5c'
    });
  });

  canvas.addEventListener('contextmenu', e => e.preventDefault());

  function loop() {
    ctx.clearRect(0, 0, canvas.width, canvas.height);

    // Draw grid lines
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.04)';
    ctx.lineWidth = 1;
    for (let x = 0; x < canvas.width; x += 40) {
      ctx.beginPath(); ctx.moveTo(x, 0); ctx.lineTo(x, canvas.height); ctx.stroke();
    }
    for (let y = 0; y < canvas.height; y += 40) {
      ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(canvas.width, y); ctx.stroke();
    }

    // Render Ripples
    for (let i = ripples.length - 1; i >= 0; i--) {
      const r = ripples[i];
      r.radius += 1.8;
      r.opacity -= 0.035;

      if (r.opacity <= 0) {
        ripples.splice(i, 1);
        continue;
      }

      ctx.save();
      // Outer ring
      ctx.beginPath();
      ctx.arc(r.x, r.y, r.radius, 0, Math.PI * 2);
      ctx.strokeStyle = r.color;
      ctx.globalAlpha = r.opacity;
      ctx.lineWidth = 2.5;
      ctx.stroke();

      // Inner faint ring
      ctx.beginPath();
      ctx.arc(r.x, r.y, r.radius * 0.5, 0, Math.PI * 2);
      ctx.strokeStyle = r.color;
      ctx.globalAlpha = r.opacity * 0.5;
      ctx.lineWidth = 1.5;
      ctx.stroke();

      ctx.restore();
    }

    // Render Spotlight Halo around cursor
    if (mouse.active) {
      ctx.save();
      const haloGrad = ctx.createRadialGradient(mouse.x, mouse.y, 4, mouse.x, mouse.y, 38);
      haloGrad.addColorStop(0, 'rgba(255, 59, 92, 0.45)');
      haloGrad.addColorStop(0.5, 'rgba(255, 159, 67, 0.2)');
      haloGrad.addColorStop(1, 'rgba(255, 59, 92, 0)');

      ctx.fillStyle = haloGrad;
      ctx.beginPath();
      ctx.arc(mouse.x, mouse.y, 38, 0, Math.PI * 2);
      ctx.fill();

      // Simulated macOS cursor arrow
      ctx.fillStyle = '#ffffff';
      ctx.strokeStyle = '#000000';
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      ctx.moveTo(mouse.x, mouse.y);
      ctx.lineTo(mouse.x, mouse.y + 16);
      ctx.lineTo(mouse.x + 4.5, mouse.y + 12);
      ctx.lineTo(mouse.x + 10, mouse.y + 11.5);
      ctx.closePath();
      ctx.fill();
      ctx.stroke();

      ctx.restore();
    }

    requestAnimationFrame(loop);
  }

  loop();
}

// 6. Trim & Review Timeline Simulator
function initTrimTimeline() {
  const container = document.getElementById('trim-track');
  const leftHandle = document.getElementById('trim-h-left');
  const rightHandle = document.getElementById('trim-h-right');
  const info = document.getElementById('trim-info');

  if (!container || !leftHandle || !rightHandle) return;

  let leftPct = 12;
  let rightPct = 85;

  function update() {
    leftHandle.style.left = `${leftPct}%`;
    rightHandle.style.left = `${rightPct}%`;

    const leftOverlay = document.getElementById('trim-ov-left');
    const rightOverlay = document.getElementById('trim-ov-right');

    if (leftOverlay) leftOverlay.style.width = `${leftPct}%`;
    if (rightOverlay) {
      rightOverlay.style.left = `${rightPct}%`;
      rightOverlay.style.width = `${100 - rightPct}%`;
    }

    const totalFrames = 120;
    const startF = Math.round((leftPct / 100) * totalFrames);
    const endF = Math.round((rightPct / 100) * totalFrames);
    const count = endF - startF;
    const dur = (count / 15).toFixed(1);

    if (info) {
      info.textContent = `Frames: ${startF} → ${endF} (${count} frames, ${dur}s loop at 15 FPS)`;
    }
  }

  update();
}
