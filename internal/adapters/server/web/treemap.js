class TreemapEngine {
  constructor(canvasId, tooltipId) {
    this.canvas = document.getElementById(canvasId);
    this.ctx = this.canvas.getContext('2d');
    this.tooltip = document.getElementById(tooltipId);
    this.nodes = [];
    this.hoveredNode = null;
    this.highlightQuery = '';
    this.onSelect = null;

    this.colors = [
      '#6366f1', '#10b981', '#f59e0b', '#0ea5e9', '#8b5cf6',
      '#06b6d4', '#ec4899', '#14b8a6', '#f43f5e', '#a855f7'
    ];

    this.initEvents();
  }

  initEvents() {
    window.addEventListener('resize', () => this.resizeAndDraw());

    this.canvas.addEventListener('mousemove', (e) => {
      const rect = this.canvas.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;

      const hit = this.hitTest(x, y);
      if (hit !== this.hoveredNode) {
        this.hoveredNode = hit;
        this.draw();
        this.updateTooltip(hit, e.clientX, e.clientY);
      } else if (hit) {
        this.updateTooltip(hit, e.clientX, e.clientY);
      }
    });

    this.canvas.addEventListener('mouseleave', () => {
      this.hoveredNode = null;
      this.tooltip.classList.add('hidden');
      this.draw();
    });

    this.canvas.addEventListener('click', (e) => {
      const rect = this.canvas.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;
      const hit = this.hitTest(x, y);
      if (hit && !hit.item.isOther && typeof this.onSelect === 'function') {
        this.onSelect(hit.item);
      }
    });
  }

  resizeAndDraw() {
    if (!this.canvas || !this.canvas.parentElement) return;
    const rect = this.canvas.parentElement.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    this.canvas.width = rect.width * dpr;
    this.canvas.height = rect.height * dpr;
    this.ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    this.displayWidth = rect.width;
    this.displayHeight = rect.height;

    this.computeLayout();
    this.draw();
  }

  setData(packages, totalBytes) {
    this.rawPackages = packages || [];
    this.totalBytes = totalBytes || this.rawPackages.reduce((acc, p) => acc + (p.sizeBytes || 0), 0);

    // LoD aggregation: combine packages < 0.2% of total into "Others"
    const threshold = this.totalBytes * 0.002;
    const major = [];
    let otherBytes = 0;
    let otherCount = 0;

    for (const p of this.rawPackages) {
      if (p.sizeBytes >= threshold) {
        major.push({ ...p });
      } else {
        otherBytes += p.sizeBytes;
        otherCount++;
      }
    }

    if (otherCount > 0) {
      major.push({
        name: `(Others: ${otherCount} packages)`,
        sizeBytes: otherBytes,
        isOther: true
      });
    }

    this.data = major;
    this.resizeAndDraw();
  }

  computeLayout() {
    if (!this.data || this.data.length === 0 || !this.displayWidth) return;
    this.nodes = [];

    // Slice-and-dice squarified division for canvas
    let remaining = [...this.data];
    let x = 0, y = 0, w = this.displayWidth, h = this.displayHeight;

    const total = remaining.reduce((sum, item) => sum + item.sizeBytes, 0);
    if (total === 0) return;

    for (let i = 0; i < remaining.length; i++) {
      const item = remaining[i];
      const ratio = item.sizeBytes / total;

      let nw, nh;
      if (w > h) {
        nw = Math.max(2, w * ratio);
        nh = h;
        this.nodes.push({ item, x, y, w: nw, h: nh });
        x += nw;
        w -= nw;
      } else {
        nw = w;
        nh = Math.max(2, h * ratio);
        this.nodes.push({ item, x, y, w: nw, h: nh });
        y += nh;
        h -= nh;
      }
    }
  }

  draw() {
    if (!this.ctx || !this.displayWidth) return;
    this.ctx.clearRect(0, 0, this.displayWidth, this.displayHeight);

    for (let i = 0; i < this.nodes.length; i++) {
      const node = this.nodes[i];
      const isHovered = (node === this.hoveredNode);
      const isMatch = this.highlightQuery && node.item.name.toLowerCase().includes(this.highlightQuery);

      this.ctx.save();
      const baseColor = this.colors[i % this.colors.length];
      this.ctx.fillStyle = node.item.isOther ? '#334155' : baseColor;

      if (this.highlightQuery && !isMatch) {
        this.ctx.globalAlpha = 0.2;
      } else {
        this.ctx.globalAlpha = isHovered ? 1.0 : 0.85;
      }

      this.ctx.fillRect(node.x, node.y, node.w, node.h);

      // Borders
      this.ctx.strokeStyle = isHovered ? '#ffffff' : '#0f172a';
      this.ctx.lineWidth = isHovered ? 2 : 1;
      this.ctx.strokeRect(node.x, node.y, node.w, node.h);

      // Text label if block is big enough
      if (node.w > 40 && node.h > 20) {
        this.ctx.fillStyle = '#ffffff';
        this.ctx.font = '11px monospace';
        const label = node.item.name;
        this.ctx.fillText(label, node.x + 6, node.y + 16, Math.max(0, node.w - 12));
      }

      this.ctx.restore();
    }
  }

  hitTest(x, y) {
    for (const node of this.nodes) {
      if (x >= node.x && x <= node.x + node.w && y >= node.y && y <= node.y + node.h) {
        return node;
      }
    }
    return null;
  }

  updateTooltip(node, clientX, clientY) {
    if (!node) {
      this.tooltip.classList.add('hidden');
      return;
    }
    const pct = this.totalBytes > 0 ? ((node.item.sizeBytes / this.totalBytes) * 100).toFixed(1) : '0.0';
    this.tooltip.innerHTML = `
      <div class="font-bold text-white">${node.item.name}</div>
      <div class="text-slate-300">Size: ${this.formatBytes(node.item.sizeBytes)} (${pct}%)</div>
      ${node.item.gzipBytes ? `<div class="text-slate-400">Gzip: ~${this.formatBytes(node.item.gzipBytes)}</div>` : ''}
    `;
    this.tooltip.style.left = `${clientX + 12}px`;
    this.tooltip.style.top = `${clientY + 12}px`;
    this.tooltip.classList.remove('hidden');
  }

  formatBytes(b) {
    if (b >= 1048576) return (b / 1048576).toFixed(2) + ' MB';
    if (b >= 1024) return (b / 1024).toFixed(2) + ' KB';
    return (b || 0) + ' B';
  }

  setHighlight(query) {
    this.highlightQuery = (query || '').toLowerCase().trim();
    this.draw();
  }
}
