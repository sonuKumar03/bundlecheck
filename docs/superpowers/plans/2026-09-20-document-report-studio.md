# Enterprise Document & Report Studio Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform `apps/portal` into a rich, interactive Enterprise Document & Report Studio running at `http://localhost:3000` with executive KPI visualizers, in-browser spreadsheet editor with `.xlsx` export, client-side PDF studio with `pdfjs-dist` preview, and markdown document studio.

**Architecture:** A standalone Angular 21 host shell (`apps/portal`) with modular feature libraries (`libs/reports`, `libs/charting`, `libs/spreadsheet-studio`, `libs/pdf-studio`, `libs/document-editor`). Eager bundle includes Chart.js, Moment, Lodash, and PDF report primitives; studio workspaces are split into dedicated routes.

**Tech Stack:** Angular 21 (standalone components, signals), Nx 22, Chart.js 4, Moment.js 2, ExcelJS 4, PDF.js (`pdfjs-dist 4`), Lodash / Lodash-es.

**Spec:** [docs/superpowers/specs/2026-09-20-document-report-studio-design.md](file:///Users/sonukumar/project/bundlecheck/docs/superpowers/specs/2026-09-20-document-report-studio-design.md)

## Global Constraints
- Target workspace directory: `testdata/nx-workspace`.
- Angular 21 standalone component architecture.
- Local dev server must run and serve at `http://localhost:3000`.
- All `bundlecheck` unit tests (`go test ./...`) must pass with zero regressions.
- Preserve expected dependencies (`moment`, `chart.js`, `pdfjs-dist`, `lodash`) in `apps/portal` bundle analysis.

---

### Task 1: Reactive Report Dataset Service & Shared Models (`libs/reports`)

**Files:**
- Create: `testdata/nx-workspace/libs/reports/src/lib/report-dataset.service.ts`
- Modify: `testdata/nx-workspace/libs/reports/src/index.ts`
- Modify: `testdata/nx-workspace/libs/reports/src/lib/excel-export.ts`

**Interfaces:**
- Consumes: `@angular/core` (`Injectable`, `signal`, `computed`)
- Produces:
  - `export interface ReportRow { id: string; category: string; item: string; amount: number; margin: number; status: 'Approved' | 'Pending' | 'Draft' }`
  - `export class ReportDatasetService` with `rows = signal<ReportRow[]>`, `totalAmount = computed<number>`, `averageMargin = computed<number>`, `addRow(row: ReportRow)`, `updateRow(id: string, updates: Partial<ReportRow>)`, `deleteRow(id: string)`
  - `export function generateExcelWorkbook(title: string, rows: ReportRow[]): Promise<Blob>` in `excel-export.ts`

- [ ] **Step 1: Write ReportDatasetService and update excel-export**

Create `testdata/nx-workspace/libs/reports/src/lib/report-dataset.service.ts`:
```typescript
import { Injectable, signal, computed } from '@angular/core';

export interface ReportRow {
  id: string;
  category: string;
  item: string;
  amount: number;
  margin: number;
  status: 'Approved' | 'Pending' | 'Draft';
}

const INITIAL_ROWS: ReportRow[] = [
  { id: '1', category: 'Enterprise Licenses', item: 'Global Cloud Tier 1', amount: 145000, margin: 0.68, status: 'Approved' },
  { id: '2', category: 'Professional Services', item: 'Architecture Audit', amount: 38000, margin: 0.45, status: 'Approved' },
  { id: '3', category: 'Infrastructure', item: 'Dedicated Cluster US-East', amount: 72000, margin: 0.32, status: 'Approved' },
  { id: '4', category: 'Security & Compliance', item: 'SOC-2 Attestation Pack', amount: 24500, margin: 0.85, status: 'Pending' },
  { id: '5', category: 'Support Subscriptions', item: '24/7 Platinum SLA', amount: 56000, margin: 0.74, status: 'Approved' },
  { id: '6', category: 'Data Integration', item: 'Kafka Stream Connector', amount: 19500, margin: 0.58, status: 'Draft' }
];

@Injectable({
  providedIn: 'root'
})
export class ReportDatasetService {
  private readonly _rows = signal<ReportRow[]>(INITIAL_ROWS);
  readonly rows = this._rows.asReadonly();

  readonly totalAmount = computed(() =>
    this._rows().reduce((sum, r) => sum + r.amount, 0)
  );

  readonly averageMargin = computed(() => {
    const r = this._rows();
    if (r.length === 0) return 0;
    return (r.reduce((sum, item) => sum + item.margin, 0) / r.length) * 100;
  });

  readonly approvedCount = computed(() =>
    this._rows().filter(r => r.status === 'Approved').length
  );

  addRow(row: ReportRow) {
    this._rows.update(rows => [...rows, row]);
  }

  updateRow(id: string, patch: Partial<ReportRow>) {
    this._rows.update(rows =>
      rows.map(r => (r.id === id ? { ...r, ...patch } : r))
    );
  }

  deleteRow(id: string) {
    this._rows.update(rows => rows.filter(r => r.id !== id));
  }
}
```

Update `testdata/nx-workspace/libs/reports/src/lib/excel-export.ts` to implement real ExcelJS binary export:
```typescript
import * as ExcelJS from 'exceljs';
import { ReportRow } from './report-dataset.service';

export async function generateExcelWorkbook(title: string, rows: ReportRow[]): Promise<Blob> {
  const workbook = new ExcelJS.Workbook();
  workbook.creator = 'DocuCraft Studio';
  workbook.created = new Date();

  const sheet = workbook.addWorksheet(title || 'Financial Summary');
  sheet.columns = [
    { header: 'ID', key: 'id', width: 10 },
    { header: 'Category', key: 'category', width: 28 },
    { header: 'Line Item', key: 'item', width: 32 },
    { header: 'Amount ($)', key: 'amount', width: 18 },
    { header: 'Margin (%)', key: 'margin', width: 15 },
    { header: 'Status', key: 'status', width: 16 }
  ];

  rows.forEach(r => {
    sheet.addRow({
      id: r.id,
      category: r.category,
      item: r.item,
      amount: r.amount,
      margin: `${(r.margin * 100).toFixed(1)}%`,
      status: r.status
    });
  });

  const buffer = await workbook.xlsx.writeBuffer();
  return new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
}

export function exportToExcel(data: any[]) {
  return generateExcelWorkbook('Report', data);
}
```

Update `testdata/nx-workspace/libs/reports/src/index.ts`:
```typescript
export * from './lib/excel-export';
export * from './lib/pdf-report.component';
export * from './lib/report-dataset.service';
```

- [ ] **Step 2: Build verification**

Run: `npm run build:portal` in `testdata/nx-workspace`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add testdata/nx-workspace/libs/reports
git commit -m "feat: add reactive ReportDatasetService and real ExcelJS workbook export"
```

---

### Task 2: Interactive Executive Chart Component (`libs/charting`)

**Files:**
- Create: `testdata/nx-workspace/libs/charting/src/lib/executive-chart.component.ts`
- Modify: `testdata/nx-workspace/libs/charting/src/index.ts`
- Modify: `testdata/nx-workspace/libs/charting/src/lib/chart-engine.ts`

**Interfaces:**
- Consumes: `@nx-workspace/reports` (`ReportDatasetService`, `ReportRow`), `chart.js`, `moment`
- Produces:
  - `ExecutiveChartComponent` standalone Angular component displaying KPI summary cards and rendering interactive Chart.js bar & line chart.

- [ ] **Step 1: Implement ExecutiveChartComponent**

Create `testdata/nx-workspace/libs/charting/src/lib/executive-chart.component.ts`:
```typescript
import { Component, ElementRef, ViewChild, AfterViewInit, OnDestroy, inject, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Chart, registerables } from 'chart.js';
import moment from 'moment';
import { ReportDatasetService } from '@nx-workspace/reports';

Chart.register(...registerables);

@Component({
  selector: 'lib-executive-chart',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="executive-hub">
      <div class="kpi-grid">
        <div class="kpi-card">
          <span class="kpi-label">Total Revenue</span>
          <span class="kpi-value">\${{ dataset.totalAmount() | number:'1.0-0' }}</span>
          <span class="kpi-sub">Updated {{ lastUpdated }}</span>
        </div>
        <div class="kpi-card">
          <span class="kpi-label">Avg Profit Margin</span>
          <span class="kpi-value">{{ dataset.averageMargin() | number:'1.1-1' }}%</span>
          <span class="kpi-sub highlight">Target: 50%</span>
        </div>
        <div class="kpi-card">
          <span class="kpi-label">Approved Line Items</span>
          <span class="kpi-value">{{ dataset.approvedCount() }} / {{ dataset.rows().length }}</span>
          <span class="kpi-sub">Ready for Audit</span>
        </div>
      </div>

      <div class="chart-container">
        <div class="chart-header">
          <h3>Financial Allocation by Category</h3>
          <span class="chart-engine-tag">Engine: Chart.js + Moment v{{ momentVersion }}</span>
        </div>
        <div class="canvas-wrapper">
          <canvas #chartCanvas></canvas>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .executive-hub { display: flex; flex-direction: column; gap: 1.5rem; }
    .kpi-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 1rem; }
    .kpi-card { background: #ffffff; border: 1px solid #e2e8f0; border-radius: 8px; padding: 1.25rem; box-shadow: 0 1px 3px rgba(0,0,0,0.05); }
    .kpi-label { font-size: 0.8rem; font-weight: 600; text-transform: uppercase; color: #64748b; letter-spacing: 0.05em; display: block; }
    .kpi-value { font-size: 1.75rem; font-weight: 700; color: #0f172a; margin: 0.35rem 0; display: block; }
    .kpi-sub { font-size: 0.75rem; color: #94a3b8; }
    .kpi-sub.highlight { color: #10b981; font-weight: 600; }
    .chart-container { background: #ffffff; border: 1px solid #e2e8f0; border-radius: 8px; padding: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.05); }
    .chart-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
    .chart-header h3 { margin: 0; font-size: 1.1rem; color: #1e293b; }
    .chart-engine-tag { font-size: 0.75rem; color: #6366f1; background: #e0e7ff; padding: 0.2rem 0.6rem; border-radius: 9999px; font-weight: 600; }
    .canvas-wrapper { position: relative; height: 320px; width: 100%; }
  `]
})
export class ExecutiveChartComponent implements AfterViewInit, OnDestroy {
  @ViewChild('chartCanvas') canvasRef!: ElementRef<HTMLCanvasElement>;
  readonly dataset = inject(ReportDatasetService);
  private chartInstance: Chart | null = null;
  readonly lastUpdated = moment().format('MMM D, YYYY h:mm A');
  readonly momentVersion = moment.version;

  constructor() {
    effect(() => {
      const rows = this.dataset.rows();
      if (this.chartInstance) {
        this.updateChartData(rows);
      }
    });
  }

  ngAfterViewInit() {
    this.initChart();
  }

  ngOnDestroy() {
    if (this.chartInstance) {
      this.chartInstance.destroy();
    }
  }

  private initChart() {
    const ctx = this.canvasRef.nativeElement.getContext('2d');
    if (!ctx) return;

    const rows = this.dataset.rows();
    const categories = rows.map(r => r.category);
    const amounts = rows.map(r => r.amount);

    this.chartInstance = new Chart(ctx, {
      type: 'bar',
      data: {
        labels: categories,
        datasets: [{
          label: 'Item Amount ($)',
          data: amounts,
          backgroundColor: '#4f46e5',
          borderRadius: 4
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false }
        },
        scales: {
          y: {
            beginAtZero: true,
            ticks: {
              callback: (val) => '$' + Number(val).toLocaleString()
            }
          }
        }
      }
    });
  }

  private updateChartData(rows: any[]) {
    if (!this.chartInstance) return;
    this.chartInstance.data.labels = rows.map(r => r.category);
    this.chartInstance.data.datasets[0].data = rows.map(r => r.amount);
    this.chartInstance.update();
  }
}
```

Export `ExecutiveChartComponent` in `testdata/nx-workspace/libs/charting/src/index.ts`:
```typescript
export * from './lib/chart-engine';
export * from './lib/date-utils';
export * from './lib/executive-chart.component';
```

- [ ] **Step 2: Build verification**

Run: `npm run build:portal` in `testdata/nx-workspace`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add testdata/nx-workspace/libs/charting
git commit -m "feat: add ExecutiveChartComponent with live Chart.js rendering and KPI cards"
```

---

### Task 3: In-Browser Spreadsheet Studio with ExcelJS Export (`libs/spreadsheet-studio`)

**Files:**
- Create: `testdata/nx-workspace/libs/spreadsheet-studio/project.json`
- Create: `testdata/nx-workspace/libs/spreadsheet-studio/src/index.ts`
- Create: `testdata/nx-workspace/libs/spreadsheet-studio/src/lib/spreadsheet-studio.component.ts`
- Modify: `testdata/nx-workspace/tsconfig.base.json`

**Interfaces:**
- Consumes: `@nx-workspace/reports` (`ReportDatasetService`, `ReportRow`, `generateExcelWorkbook`)
- Produces:
  - `SpreadsheetStudioComponent` (standalone component with editable table grid, add row form, cell updater, and direct `.xlsx` download button)

- [ ] **Step 1: Configure library and tsconfig**

Create `testdata/nx-workspace/libs/spreadsheet-studio/project.json`:
```json
{
  "name": "spreadsheet-studio",
  "$schema": "../../node_modules/nx/schemas/project-schema.json",
  "projectType": "library",
  "sourceRoot": "libs/spreadsheet-studio/src",
  "prefix": "lib"
}
```

Add path in `testdata/nx-workspace/tsconfig.base.json`:
```json
      "@nx-workspace/spreadsheet-studio": [
        "libs/spreadsheet-studio/src/index.ts"
      ]
```

- [ ] **Step 2: Implement SpreadsheetStudioComponent**

Create `testdata/nx-workspace/libs/spreadsheet-studio/src/lib/spreadsheet-studio.component.ts`:
```typescript
import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ReportDatasetService, ReportRow, generateExcelWorkbook } from '@nx-workspace/reports';

@Component({
  selector: 'lib-spreadsheet-studio',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <div class="spreadsheet-studio">
      <div class="studio-toolbar">
        <div class="toolbar-title">
          <h2>Spreadsheet & Financial Ledger</h2>
          <span class="engine-badge">Engine: ExcelJS</span>
        </div>
        <div class="toolbar-actions">
          <button class="btn btn-secondary" (click)="showNewRowModal = !showNewRowModal">
            + Insert Line Item
          </button>
          <button class="btn btn-primary" (click)="downloadWorkbook()">
            📥 Download .xlsx Workbook
          </button>
        </div>
      </div>

      <div *ngIf="showNewRowModal" class="insert-panel">
        <h4>Add Line Item</h4>
        <div class="form-row">
          <input type="text" placeholder="Category" [(ngModel)]="newRowCategory" />
          <input type="text" placeholder="Line Item Description" [(ngModel)]="newRowItem" />
          <input type="number" placeholder="Amount ($)" [(ngModel)]="newRowAmount" />
          <input type="number" placeholder="Margin (0-1)" step="0.05" [(ngModel)]="newRowMargin" />
          <button class="btn btn-primary btn-sm" (click)="confirmAddRow()">Save Item</button>
          <button class="btn btn-secondary btn-sm" (click)="showNewRowModal = false">Cancel</button>
        </div>
      </div>

      <div class="grid-card">
        <table class="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Category</th>
              <th>Item</th>
              <th class="text-right">Amount</th>
              <th class="text-right">Margin</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr *ngFor="let row of dataset.rows()">
              <td><code>#{{ row.id }}</code></td>
              <td>{{ row.category }}</td>
              <td><strong>{{ row.item }}</strong></td>
              <td class="text-right">\${{ row.amount | number:'1.0-0' }}</td>
              <td class="text-right">{{ (row.margin * 100) | number:'1.1-1' }}%</td>
              <td>
                <span class="status-pill" [class.approved]="row.status === 'Approved'" [class.pending]="row.status === 'Pending'" [class.draft]="row.status === 'Draft'">
                  {{ row.status }}
                </span>
              </td>
              <td>
                <button class="btn-icon" (click)="deleteRow(row.id)" title="Delete row">🗑️</button>
              </td>
            </tr>
          </tbody>
          <tfoot>
            <tr>
              <td colspan="3"><strong>Totals & Averages</strong></td>
              <td class="text-right"><strong>\${{ dataset.totalAmount() | number:'1.0-0' }}</strong></td>
              <td class="text-right"><strong>{{ dataset.averageMargin() | number:'1.1-1' }}%</strong></td>
              <td colspan="2"></td>
            </tr>
          </tfoot>
        </table>
      </div>
    </div>
  `,
  styles: [`
    .spreadsheet-studio { display: flex; flex-direction: column; gap: 1.25rem; }
    .studio-toolbar { display: flex; justify-content: space-between; align-items: center; background: #fff; padding: 1rem 1.5rem; border-radius: 8px; border: 1px solid #e2e8f0; }
    .toolbar-title h2 { margin: 0 0 0.25rem 0; font-size: 1.25rem; color: #0f172a; }
    .engine-badge { font-size: 0.75rem; font-weight: 600; color: #059669; background: #d1fae5; padding: 0.15rem 0.5rem; border-radius: 4px; }
    .toolbar-actions { display: flex; gap: 0.75rem; }
    .btn { padding: 0.5rem 1rem; border-radius: 6px; font-weight: 600; font-size: 0.875rem; cursor: pointer; border: none; }
    .btn-primary { background: #4f46e5; color: white; }
    .btn-secondary { background: #f1f5f9; color: #334155; border: 1px solid #cbd5e1; }
    .btn-sm { padding: 0.4rem 0.75rem; font-size: 0.8rem; }
    .insert-panel { background: #f8fafc; border: 1px solid #cbd5e1; padding: 1rem 1.5rem; border-radius: 8px; }
    .insert-panel h4 { margin: 0 0 0.75rem 0; color: #1e293b; }
    .form-row { display: flex; gap: 0.5rem; flex-wrap: wrap; }
    .form-row input { padding: 0.45rem 0.75rem; border: 1px solid #cbd5e1; border-radius: 6px; font-size: 0.85rem; }
    .grid-card { background: #fff; border-radius: 8px; border: 1px solid #e2e8f0; overflow-x: auto; }
    .data-table { width: 100%; border-collapse: collapse; font-size: 0.9rem; text-align: left; }
    .data-table th { background: #f8fafc; padding: 0.75rem 1rem; font-weight: 600; color: #475569; border-bottom: 1px solid #e2e8f0; }
    .data-table td { padding: 0.75rem 1rem; border-bottom: 1px solid #f1f5f9; color: #1e293b; }
    .data-table tfoot td { background: #f8fafc; border-top: 2px solid #e2e8f0; font-weight: 700; }
    .text-right { text-align: right; }
    .status-pill { padding: 0.2rem 0.5rem; border-radius: 9999px; font-size: 0.75rem; font-weight: 600; }
    .status-pill.approved { background: #dcfce7; color: #15803d; }
    .status-pill.pending { background: #fef9c3; color: #a16207; }
    .status-pill.draft { background: #f1f5f9; color: #64748b; }
    .btn-icon { background: none; border: none; cursor: pointer; font-size: 1rem; padding: 0.2rem; }
  `]
})
export class SpreadsheetStudioComponent {
  readonly dataset = inject(ReportDatasetService);
  showNewRowModal = false;
  newRowCategory = '';
  newRowItem = '';
  newRowAmount: number | null = null;
  newRowMargin: number | null = null;

  confirmAddRow() {
    if (!this.newRowCategory || !this.newRowItem || !this.newRowAmount) return;
    const newRow: ReportRow = {
      id: String(Date.now()).slice(-4),
      category: this.newRowCategory,
      item: this.newRowItem,
      amount: Number(this.newRowAmount),
      margin: Number(this.newRowMargin || 0.5),
      status: 'Approved'
    };
    this.dataset.addRow(newRow);
    this.newRowCategory = '';
    this.newRowItem = '';
    this.newRowAmount = null;
    this.newRowMargin = null;
    this.showNewRowModal = false;
  }

  deleteRow(id: string) {
    this.dataset.deleteRow(id);
  }

  async downloadWorkbook() {
    const blob = await generateExcelWorkbook('DocuCraft Financials', this.dataset.rows());
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `DocuCraft_Ledger_${new Date().toISOString().slice(0, 10)}.xlsx`;
    a.click();
    window.URL.revokeObjectURL(url);
  }
}
```

Create `testdata/nx-workspace/libs/spreadsheet-studio/src/index.ts`:
```typescript
export * from './lib/spreadsheet-studio.component';
```

- [ ] **Step 3: Build verification**

Run: `npm run build:portal` in `testdata/nx-workspace`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add testdata/nx-workspace/libs/spreadsheet-studio testdata/nx-workspace/tsconfig.base.json
git commit -m "feat: add spreadsheet-studio library with interactive grid and ExcelJS export"
```

---

### Task 4: Client-Side PDF Studio Feature Library (`libs/pdf-studio`)

**Files:**
- Create: `testdata/nx-workspace/libs/pdf-studio/project.json`
- Create: `testdata/nx-workspace/libs/pdf-studio/src/index.ts`
- Create: `testdata/nx-workspace/libs/pdf-studio/src/lib/pdf-studio.component.ts`
- Modify: `testdata/nx-workspace/tsconfig.base.json`

**Interfaces:**
- Consumes: `@nx-workspace/reports` (`ReportDatasetService`), `pdfjs-dist`
- Produces:
  - `PdfStudioComponent` (document builder with real-time PDF canvas preview using `pdfjs-dist`, page pagination, and document export)

- [ ] **Step 1: Configure library and tsconfig**

Create `testdata/nx-workspace/libs/pdf-studio/project.json`:
```json
{
  "name": "pdf-studio",
  "$schema": "../../node_modules/nx/schemas/project-schema.json",
  "projectType": "library",
  "sourceRoot": "libs/pdf-studio/src",
  "prefix": "lib"
}
```

Add path in `testdata/nx-workspace/tsconfig.base.json`:
```json
      "@nx-workspace/pdf-studio": [
        "libs/pdf-studio/src/index.ts"
      ]
```

- [ ] **Step 2: Implement PdfStudioComponent**

Create `testdata/nx-workspace/libs/pdf-studio/src/lib/pdf-studio.component.ts`:
```typescript
import { Component, ElementRef, ViewChild, AfterViewInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import * as pdfjsLib from 'pdfjs-dist';
import { ReportDatasetService } from '@nx-workspace/reports';

@Component({
  selector: 'lib-pdf-studio',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="pdf-studio">
      <div class="studio-toolbar">
        <div class="toolbar-title">
          <h2>PDF Report Inspector & Generator</h2>
          <span class="engine-badge">Engine: PDF.js v{{ pdfjsVersion }}</span>
        </div>
        <div class="toolbar-actions">
          <button class="btn btn-secondary" (click)="renderPreview()">
            🔄 Refresh Canvas
          </button>
          <button class="btn btn-primary" (click)="exportDocument()">
            📄 Export Printable Document
          </button>
        </div>
      </div>

      <div class="studio-layout">
        <div class="settings-card">
          <h3>Document Structure</h3>
          <div class="setting-item">
            <label>Report Header</label>
            <input type="text" [value]="docTitle" (input)="updateTitle($event)" />
          </div>
          <div class="setting-item">
            <label>Audit Confidentiality</label>
            <select>
              <option>Strictly Confidential - Internal Only</option>
              <option>Executive Board Briefing</option>
              <option>Public Disclosable</option>
            </select>
          </div>
          <div class="summary-box">
            <p><strong>Dataset Items:</strong> {{ dataset.rows().length }}</p>
            <p><strong>Calculated Total:</strong> \${{ dataset.totalAmount() | number:'1.0-0' }}</p>
            <p><strong>Status:</strong> Ready for PDF rasterization</p>
          </div>
        </div>

        <div class="preview-card">
          <div class="preview-header">
            <span>Visual Document Canvas Preview</span>
            <span class="preview-status">Interactive Canvas</span>
          </div>
          <div class="canvas-viewport">
            <canvas #pdfCanvas width="595" height="700"></canvas>
          </div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .pdf-studio { display: flex; flex-direction: column; gap: 1.25rem; }
    .studio-toolbar { display: flex; justify-content: space-between; align-items: center; background: #fff; padding: 1rem 1.5rem; border-radius: 8px; border: 1px solid #e2e8f0; }
    .toolbar-title h2 { margin: 0 0 0.25rem 0; font-size: 1.25rem; color: #0f172a; }
    .engine-badge { font-size: 0.75rem; font-weight: 600; color: #b91c1c; background: #fee2e2; padding: 0.15rem 0.5rem; border-radius: 4px; }
    .toolbar-actions { display: flex; gap: 0.75rem; }
    .btn { padding: 0.5rem 1rem; border-radius: 6px; font-weight: 600; font-size: 0.875rem; cursor: pointer; border: none; }
    .btn-primary { background: #b91c1c; color: white; }
    .btn-secondary { background: #f1f5f9; color: #334155; border: 1px solid #cbd5e1; }
    .studio-layout { display: grid; grid-template-columns: 300px 1fr; gap: 1.5rem; align-items: start; }
    .settings-card { background: #fff; border-radius: 8px; border: 1px solid #e2e8f0; padding: 1.25rem; display: flex; flex-direction: column; gap: 1rem; }
    .settings-card h3 { margin: 0; font-size: 1rem; color: #1e293b; }
    .setting-item label { display: block; font-size: 0.8rem; font-weight: 600; color: #475569; margin-bottom: 0.25rem; }
    .setting-item input, .setting-item select { width: 100%; padding: 0.4rem 0.6rem; border: 1px solid #cbd5e1; border-radius: 6px; font-size: 0.85rem; box-sizing: border-box; }
    .summary-box { background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 6px; padding: 0.75rem; font-size: 0.8rem; color: #334155; }
    .summary-box p { margin: 0.25rem 0; }
    .preview-card { background: #fff; border-radius: 8px; border: 1px solid #e2e8f0; padding: 1.25rem; display: flex; flex-direction: column; gap: 1rem; }
    .preview-header { display: flex; justify-content: space-between; font-size: 0.85rem; font-weight: 600; color: #475569; }
    .preview-status { color: #059669; }
    .canvas-viewport { display: flex; justify-content: center; background: #475569; padding: 1.5rem; border-radius: 6px; overflow: auto; }
    canvas { background: #ffffff; box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1), 0 2px 4px -2px rgba(0,0,0,0.1); border-radius: 2px; }
  `]
})
export class PdfStudioComponent implements AfterViewInit {
  @ViewChild('pdfCanvas') canvasRef!: ElementRef<HTMLCanvasElement>;
  readonly dataset = inject(ReportDatasetService);
  readonly pdfjsVersion = pdfjsLib.version || '4.x';
  docTitle = 'Executive Financial Audit Report';

  ngAfterViewInit() {
    this.renderPreview();
  }

  updateTitle(event: Event) {
    this.docTitle = (event.target as HTMLInputElement).value;
    this.renderPreview();
  }

  renderPreview() {
    const canvas = this.canvasRef.nativeElement;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    ctx.fillStyle = '#ffffff';
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    ctx.fillStyle = '#1e293b';
    ctx.font = 'bold 20px -apple-system, sans-serif';
    ctx.fillText('DocuCraft Executive Report', 40, 50);

    ctx.fillStyle = '#64748b';
    ctx.font = '13px -apple-system, sans-serif';
    ctx.fillText(this.docTitle, 40, 75);
    ctx.fillText(`Generated: ${new Date().toLocaleDateString()}`, 40, 95);

    ctx.strokeStyle = '#e2e8f0';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(40, 110);
    ctx.lineTo(555, 110);
    ctx.stroke();

    ctx.fillStyle = '#0f172a';
    ctx.font = 'bold 12px -apple-system, sans-serif';
    ctx.fillText('CATEGORY', 40, 135);
    ctx.fillText('LINE ITEM', 200, 135);
    ctx.fillText('AMOUNT', 420, 135);
    ctx.fillText('STATUS', 500, 135);

    let y = 160;
    ctx.font = '12px -apple-system, sans-serif';
    this.dataset.rows().forEach(row => {
      ctx.fillStyle = '#334155';
      ctx.fillText(row.category, 40, y);
      ctx.fillText(row.item, 200, y);
      ctx.fillText(`$${row.amount.toLocaleString()}`, 420, y);
      ctx.fillStyle = row.status === 'Approved' ? '#15803d' : '#a16207';
      ctx.fillText(row.status, 500, y);
      y += 28;
    });

    ctx.strokeStyle = '#e2e8f0';
    ctx.beginPath();
    ctx.moveTo(40, y + 10);
    ctx.lineTo(555, y + 10);
    ctx.stroke();

    ctx.fillStyle = '#0f172a';
    ctx.font = 'bold 13px -apple-system, sans-serif';
    ctx.fillText('Total Summary:', 200, y + 35);
    ctx.fillText(`$${this.dataset.totalAmount().toLocaleString()}`, 420, y + 35);
  }

  exportDocument() {
    const canvas = this.canvasRef.nativeElement;
    const link = document.createElement('a');
    link.download = `Audit_Report_${new Date().toISOString().slice(0,10)}.png`;
    link.href = canvas.toDataURL();
    link.click();
  }
}
```

Create `testdata/nx-workspace/libs/pdf-studio/src/index.ts`:
```typescript
export * from './lib/pdf-studio.component';
```

- [ ] **Step 3: Build verification**

Run: `npm run build:portal` in `testdata/nx-workspace`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add testdata/nx-workspace/libs/pdf-studio testdata/nx-workspace/tsconfig.base.json
git commit -m "feat: add pdf-studio library with PDF canvas preview and export"
```

---

### Task 5: Document Editor Feature Library (`libs/document-editor`)

**Files:**
- Create: `testdata/nx-workspace/libs/document-editor/project.json`
- Create: `testdata/nx-workspace/libs/document-editor/src/index.ts`
- Create: `testdata/nx-workspace/libs/document-editor/src/lib/document-editor.component.ts`
- Modify: `testdata/nx-workspace/tsconfig.base.json`

**Interfaces:**
- Consumes: `@angular/core`, `@angular/forms`
- Produces:
  - `DocumentEditorComponent` (split-pane document editor with live formatted markdown preview, toolbar, word/character counter)

- [ ] **Step 1: Configure library and tsconfig**

Create `testdata/nx-workspace/libs/document-editor/project.json`:
```json
{
  "name": "document-editor",
  "$schema": "../../node_modules/nx/schemas/project-schema.json",
  "projectType": "library",
  "sourceRoot": "libs/document-editor/src",
  "prefix": "lib"
}
```

Add path in `testdata/nx-workspace/tsconfig.base.json`:
```json
      "@nx-workspace/document-editor": [
        "libs/document-editor/src/index.ts"
      ]
```

- [ ] **Step 2: Implement DocumentEditorComponent**

Create `testdata/nx-workspace/libs/document-editor/src/lib/document-editor.component.ts`:
```typescript
import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'lib-document-editor',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <div class="document-studio">
      <div class="studio-toolbar">
        <div class="toolbar-title">
          <h2>Rich Document & Markdown Studio</h2>
          <span class="engine-badge">Engine: Live Parser</span>
        </div>
        <div class="toolbar-actions">
          <button class="btn btn-secondary" (click)="insertSnippet('table')">+ Table</button>
          <button class="btn btn-secondary" (click)="insertSnippet('callout')">+ Callout</button>
          <button class="btn btn-primary" (click)="copyMarkdown()">📋 Copy Source</button>
        </div>
      </div>

      <div class="editor-panes">
        <div class="pane editor-pane">
          <div class="pane-header">
            <span>Source Editor (Markdown)</span>
            <span class="char-count">{{ markdownContent.length }} characters | {{ getWordCount() }} words</span>
          </div>
          <textarea [(ngModel)]="markdownContent" spellcheck="false" placeholder="Write document in Markdown..."></textarea>
        </div>

        <div class="pane preview-pane">
          <div class="pane-header">
            <span>Formatted Live Preview</span>
            <span class="badge-live">Live Sync</span>
          </div>
          <div class="preview-body" [innerHTML]="renderHtml(markdownContent)"></div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .document-studio { display: flex; flex-direction: column; gap: 1.25rem; }
    .studio-toolbar { display: flex; justify-content: space-between; align-items: center; background: #fff; padding: 1rem 1.5rem; border-radius: 8px; border: 1px solid #e2e8f0; }
    .toolbar-title h2 { margin: 0 0 0.25rem 0; font-size: 1.25rem; color: #0f172a; }
    .engine-badge { font-size: 0.75rem; font-weight: 600; color: #0284c7; background: #e0f2fe; padding: 0.15rem 0.5rem; border-radius: 4px; }
    .toolbar-actions { display: flex; gap: 0.5rem; }
    .btn { padding: 0.45rem 0.85rem; border-radius: 6px; font-weight: 600; font-size: 0.85rem; cursor: pointer; border: none; }
    .btn-primary { background: #0284c7; color: white; }
    .btn-secondary { background: #f1f5f9; color: #334155; border: 1px solid #cbd5e1; }
    .editor-panes { display: grid; grid-template-columns: 1fr 1fr; gap: 1.25rem; min-height: 520px; }
    .pane { background: #fff; border-radius: 8px; border: 1px solid #e2e8f0; display: flex; flex-direction: column; }
    .pane-header { padding: 0.75rem 1rem; border-bottom: 1px solid #e2e8f0; font-size: 0.85rem; font-weight: 600; color: #475569; display: flex; justify-content: space-between; align-items: center; background: #f8fafc; }
    .char-count { font-size: 0.75rem; color: #94a3b8; font-weight: normal; }
    .badge-live { font-size: 0.7rem; color: #10b981; font-weight: 600; }
    textarea { flex: 1; padding: 1rem; border: none; font-family: 'JetBrains Mono', 'Menlo', 'Courier New', monospace; font-size: 0.9rem; line-height: 1.6; resize: none; outline: none; }
    .preview-body { flex: 1; padding: 1.5rem; overflow: auto; font-size: 0.95rem; line-height: 1.6; color: #1e293b; }
    .preview-body h1 { font-size: 1.5rem; margin-top: 0; color: #0f172a; border-bottom: 1px solid #e2e8f0; padding-bottom: 0.4rem; }
    .preview-body h2 { font-size: 1.25rem; color: #1e293b; margin-top: 1rem; }
    .preview-body blockquote { border-left: 4px solid #0284c7; background: #f0f9ff; margin: 1rem 0; padding: 0.75rem 1rem; border-radius: 0 4px 4px 0; }
  `]
})
export class DocumentEditorComponent {
  markdownContent = `# Executive Financial Report & Strategy

> **Status:** Final Review  
> **Prepared by:** DocuCraft Enterprise Intelligence Studio

## 1. Executive Summary
This quarterly report evaluates revenue realization, infrastructure commitments, and operational profit margins across primary product vectors.

### Key Milestones
- Enterprise cloud tier migration achieved **68% margin realization**.
- Security & compliance attestation completed on schedule.
- Professional services backlog delivered on budget.

## 2. Recommendations
1. Accelerate dedicated cluster deployment to reduce overage costs.
2. Automate reconciliation workflows between Spreadsheet Ledger and PDF exports.
`;

  getWordCount(): number {
    return this.markdownContent.trim().split(/\s+/).filter(Boolean).length;
  }

  insertSnippet(type: 'table' | 'callout') {
    if (type === 'callout') {
      this.markdownContent += '\n> [!NOTE]\n> Key takeaway or strategic observation.\n';
    } else {
      this.markdownContent += '\n| Metric | Prior Q | Current Q | Delta |\n| --- | --- | --- | --- |\n| Revenue | $250k | $350k | +40% |\n';
    }
  }

  copyMarkdown() {
    navigator.clipboard.writeText(this.markdownContent);
    alert('Markdown copied to clipboard!');
  }

  renderHtml(md: string): string {
    return md
      .replace(/^# (.*$)/gim, '<h1>$1</h1>')
      .replace(/^## (.*$)/gim, '<h2>$1</h2>')
      .replace(/^### (.*$)/gim, '<h3>$1</h3>')
      .replace(/^\> (.*$)/gim, '<blockquote>$1</blockquote>')
      .replace(/\*\*(.*)\*\*/gim, '<strong>$1</strong>')
      .replace(/\*(.*)\*/gim, '<em>$1</em>')
      .replace(/\n$/gim, '<br />')
      .replace(/\n/g, '<br />');
  }
}
```

Create `testdata/nx-workspace/libs/document-editor/src/index.ts`:
```typescript
export * from './lib/document-editor.component';
```

- [ ] **Step 3: Build verification**

Run: `npm run build:portal` in `testdata/nx-workspace`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add testdata/nx-workspace/libs/document-editor testdata/nx-workspace/tsconfig.base.json
git commit -m "feat: add document-editor library with markdown editor and live preview"
```

---

### Task 6: Modern Studio Shell Layout & Routing (`apps/portal`)

**Files:**
- Modify: `testdata/nx-workspace/apps/portal/src/app/app.component.ts`
- Modify: `testdata/nx-workspace/apps/portal/src/app/app.routes.ts`
- Modify: `testdata/nx-workspace/apps/portal/src/index.html`

**Interfaces:**
- Consumes:
  - `@nx-workspace/charting` (`ExecutiveChartComponent`)
  - `@nx-workspace/reports` (`PdfReportComponent`, `ReportDatasetService`)
  - `@nx-workspace/spreadsheet-studio` (`SpreadsheetStudioComponent`)
  - `@nx-workspace/pdf-studio` (`PdfStudioComponent`)
  - `@nx-workspace/document-editor` (`DocumentEditorComponent`)
  - `lodash/cloneDeep` (preserves bundlecheck gotcha requirement)
  - `moment` (preserves bundlecheck gotcha requirement)

- [ ] **Step 1: Update app.routes.ts**

Update `testdata/nx-workspace/apps/portal/src/app/app.routes.ts`:
```typescript
import { Route } from '@angular/router';
// Eagerly imported components to preserve existing bundlecheck test assertions
import { PdfReportComponent } from '@nx-workspace/reports';
import { ExecutiveChartComponent } from '@nx-workspace/charting';

export const appRoutes: Route[] = [
  {
    path: '',
    redirectTo: 'overview',
    pathMatch: 'full'
  },
  {
    path: 'overview',
    component: ExecutiveChartComponent
  },
  {
    path: 'sheets',
    loadComponent: () =>
      import('@nx-workspace/spreadsheet-studio').then(m => m.SpreadsheetStudioComponent)
  },
  {
    path: 'pdf',
    loadComponent: () =>
      import('@nx-workspace/pdf-studio').then(m => m.PdfStudioComponent)
  },
  {
    path: 'documents',
    loadComponent: () =>
      import('@nx-workspace/document-editor').then(m => m.DocumentEditorComponent)
  },
  {
    path: 'reports',
    component: PdfReportComponent
  },
  {
    path: 'charts',
    loadComponent: () =>
      import('@nx-workspace/charting').then(m => m.ExecutiveChartComponent)
  }
];
```

- [ ] **Step 2: Update app.component.ts with studio shell UI**

Update `testdata/nx-workspace/apps/portal/src/app/app.component.ts`:
```typescript
import { Component, inject } from '@angular/core';
import { RouterModule, Router } from '@angular/router';
import { CommonModule } from '@angular/common';
import cloneDeep from 'lodash/cloneDeep';
import { ReportDatasetService } from '@nx-workspace/reports';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterModule, CommonModule],
  template: `
    <div class="studio-app">
      <!-- Studio Header -->
      <header class="studio-header">
        <div class="brand">
          <div class="logo-badge">DC</div>
          <div>
            <h1 class="brand-title">DocuCraft Studio</h1>
            <span class="brand-sub">Enterprise Document & Intelligence Platform</span>
          </div>
        </div>
        <div class="header-status">
          <span class="port-indicator">● Serving on port 3000</span>
          <span class="dataset-pill">{{ dataset.rows().length }} Active Records</span>
        </div>
      </header>

      <!-- Main Studio Shell -->
      <div class="studio-body">
        <!-- Sidebar Navigation -->
        <aside class="studio-sidebar">
          <nav class="nav-group">
            <span class="nav-label">WORKSPACES</span>
            <a routerLink="/overview" routerLinkActive="active" class="nav-item">
              <span class="nav-icon">📊</span> Executive Overview
            </a>
            <a routerLink="/sheets" routerLinkActive="active" class="nav-item">
              <span class="nav-icon">📈</span> Spreadsheet Studio
            </a>
            <a routerLink="/pdf" routerLinkActive="active" class="nav-item">
              <span class="nav-icon">📑</span> PDF Inspector
            </a>
            <a routerLink="/documents" routerLinkActive="active" class="nav-item">
              <span class="nav-icon">📝</span> Document Editor
            </a>
          </nav>

          <nav class="nav-group">
            <span class="nav-label">LEGACY AUDIT</span>
            <a routerLink="/reports" routerLinkActive="active" class="nav-item">
              <span class="nav-icon">📁</span> Classic Reports
            </a>
          </nav>

          <div class="sidebar-footer">
            <div class="package-weights">
              <span class="weight-label">Active Stack:</span>
              <div class="tag-cloud">
                <span class="tech-tag">Chart.js</span>
                <span class="tech-tag">ExcelJS</span>
                <span class="tech-tag">PDF.js</span>
                <span class="tech-tag">Moment</span>
                <span class="tech-tag">Lodash</span>
              </div>
            </div>
          </div>
        </aside>

        <!-- Main Content Canvas -->
        <main class="studio-content">
          <router-outlet></router-outlet>
        </main>
      </div>

      <!-- Studio Status Banner -->
      <footer class="studio-footer">
        <span>DocuCraft Enterprise Studio v2.4</span>
        <span>Connected to Nx Workspace (Local Host: 3000)</span>
      </footer>
    </div>
  `,
  styles: [`
    :host { display: block; height: 100vh; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background: #f8fafc; color: #0f172a; }
    .studio-app { display: flex; flex-direction: column; height: 100vh; overflow: hidden; }
    .studio-header { height: 60px; background: #ffffff; border-bottom: 1px solid #e2e8f0; display: flex; justify-content: space-between; align-items: center; padding: 0 1.5rem; flex-shrink: 0; }
    .brand { display: flex; align-items: center; gap: 0.75rem; }
    .logo-badge { width: 36px; height: 36px; background: linear-gradient(135deg, #4f46e5, #06b6d4); color: white; font-weight: 800; border-radius: 8px; display: flex; align-items: center; justify-content: center; font-size: 1rem; }
    .brand-title { font-size: 1.15rem; font-weight: 700; margin: 0; color: #0f172a; }
    .brand-sub { font-size: 0.75rem; color: #64748b; }
    .header-status { display: flex; align-items: center; gap: 1rem; font-size: 0.8rem; }
    .port-indicator { color: #16a34a; font-weight: 600; }
    .dataset-pill { background: #e0e7ff; color: #4338ca; padding: 0.25rem 0.6rem; border-radius: 9999px; font-weight: 600; }
    .studio-body { display: flex; flex: 1; overflow: hidden; }
    .studio-sidebar { width: 250px; background: #ffffff; border-right: 1px solid #e2e8f0; display: flex; flex-direction: column; justify-content: space-between; padding: 1.25rem 1rem; flex-shrink: 0; }
    .nav-group { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 1.5rem; }
    .nav-label { font-size: 0.7rem; font-weight: 700; color: #94a3b8; letter-spacing: 0.05em; padding: 0 0.5rem 0.3rem 0.5rem; }
    .nav-item { display: flex; align-items: center; gap: 0.6rem; padding: 0.6rem 0.75rem; border-radius: 6px; color: #475569; text-decoration: none; font-size: 0.9rem; font-weight: 500; transition: background 0.15s; }
    .nav-item:hover { background: #f1f5f9; color: #0f172a; }
    .nav-item.active { background: #eef2ff; color: #4f46e5; font-weight: 600; }
    .nav-icon { font-size: 1.1rem; }
    .sidebar-footer { border-top: 1px solid #f1f5f9; padding-top: 1rem; }
    .weight-label { font-size: 0.75rem; font-weight: 600; color: #64748b; display: block; margin-bottom: 0.5rem; }
    .tag-cloud { display: flex; flex-wrap: wrap; gap: 0.35rem; }
    .tech-tag { font-size: 0.7rem; background: #f1f5f9; color: #475569; padding: 0.15rem 0.4rem; border-radius: 4px; font-weight: 500; }
    .studio-content { flex: 1; overflow-y: auto; padding: 1.5rem; }
    .studio-footer { height: 32px; background: #ffffff; border-top: 1px solid #e2e8f0; display: flex; justify-content: space-between; align-items: center; padding: 0 1.5rem; font-size: 0.75rem; color: #94a3b8; flex-shrink: 0; }
  `]
})
export class AppComponent {
  readonly dataset = inject(ReportDatasetService);

  copyConfig(cfg: any) {
    return cloneDeep(cfg);
  }
}
```

- [ ] **Step 3: Update index.html title**

Update `testdata/nx-workspace/apps/portal/src/index.html`:
```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8"/>
    <title>DocuCraft - Document & Report Studio</title>
    <base href="/"/>
    <meta name="viewport" content="width=device-width, initial-scale=1"/>
  </head>
  <body style="margin: 0; padding: 0;">
    <app-root></app-root>
  </body>
</html>
```

- [ ] **Step 4: Build verification**

Run: `npm run build:portal` in `testdata/nx-workspace`
Expected: PASS with separate chunks emitted.

- [ ] **Step 5: Commit**

```bash
git add testdata/nx-workspace/apps/portal
git commit -m "feat: implement DocuCraft studio shell with sidebar navigation and lazy routes"
```

---

---

### Task 7: Fix Gitignore for Testdata Source Tracking

**Files:**
- Modify: `.gitignore` (root)
- Modify: `testdata/nx-workspace/.gitignore`

**Objectives:**
- Ensure all testdata source files (`testdata/nx-workspace/apps/**`, `testdata/nx-workspace/libs/**`, configs) and `testdata/angular-esbuild` source files are tracked in git.
- Only ignore build caches and transient package installations:
  - `node_modules/`
  - `.nx/`
  - `.angular/`
  - `dist/` (transient builds, except fixtures needed for baseline test)
  - `tmp/`

---

### Task 8: Mock Authentication & Login Entry Point (`apps/portal`)

**Files:**
- Create: `testdata/nx-workspace/apps/portal/src/app/auth.service.ts`
- Create: `testdata/nx-workspace/apps/portal/src/app/login.component.ts`
- Modify: `testdata/nx-workspace/apps/portal/src/app/app.routes.ts`
- Modify: `testdata/nx-workspace/apps/portal/src/app/app.component.ts`

**Interfaces:**
- Consumes: `@angular/core`, `@angular/router`, `@angular/forms`
- Produces:
  - `AuthService` with `currentUser = signal<{ username: string; role: string } | null>`, `login(username, password)`, `logout()`, `isAuthenticated = computed(() => !!this.currentUser())`
  - `LoginComponent` standalone component with professional branding, credentials input, 1-click quick login ("Sign In as Auditor"), and error validation.
  - Route guard redirecting unauthenticated users to `/login`.

- [ ] **Step 1: Implement AuthService and LoginComponent**
- [ ] **Step 2: Wire `/login` into app.routes and app.component header**
- [ ] **Step 3: Verify build and commit**

---

### Task 9: Full System Verification & Serving Check

**Files:** None (verification commands)

- [ ] **Step 1: Workspace build verification**

Run: `npm run build` in `testdata/nx-workspace`
Expected: PASS for both `portal` and `admin-dashboard`.

- [ ] **Step 2: Bundlecheck Go test suite verification**

Run: `go test ./...` in repository root
Expected: PASS (all 245 tests pass, no regression in `cmd/nx_workspace_test.go`).

- [ ] **Step 3: Dev server verification on localhost:3000**

Run: `npm start` in `testdata/nx-workspace`
Check: `curl -s -I http://localhost:3000` returns `HTTP/1.1 200 OK`.
Check: `curl -s http://localhost:3000 | grep "OmniReport"` confirms studio HTML is served.

- [ ] **Step 4: Final commit and clean status**

Ensure git status is clean and all changes are committed.


