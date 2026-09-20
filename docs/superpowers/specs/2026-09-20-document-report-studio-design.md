# Enterprise Document & Report Studio Design

## Overview
Transform `apps/portal` in `testdata/nx-workspace` into an **Enterprise Document & Report Studio** served at `http://localhost:3000`. The application serves as an interactive, production-grade corporate reporting suite and an ideal testbed for dependency inspection with `bundlecheck`.

The architecture intentionally leverages heavy, realistic npm dependencies (`pdfjs-dist`, `exceljs`, `chart.js`, `d3`, `moment`, `lodash-es`) split across dedicated Nx feature libraries and Angular lazy-loaded routes.

---

## Goals & Non-Goals

### Goals
- **Real Interactive Studio Application**: Serve a functional studio application at `http://localhost:3000` with executive KPI cards, an interactive spreadsheet editor with Excel export, a PDF document generator and previewer, and a rich document formatting workspace.
- **Realistic Heavy Package Architecture**:
  - Eager initial bundle containing `chart.js`, `moment`, `lodash-es`, and core routing/navigation.
  - Lazy-loaded feature routes splitting `exceljs`, `pdfjs-dist`, and specialized report modules.
- **Modular Nx Workspace Structure**: Clean library boundaries using existing and new Nx libs (`libs/charting`, `libs/reports`, `libs/spreadsheet-studio`, `libs/pdf-studio`, `libs/document-editor`).
- **Bundlecheck & Nx Tooling Compatibility**: Preserve 100% test compatibility with `bundlecheck` fixtures, `stats.json` emissions, and Go unit tests.

### Non-Goals
- Full multi-tenant authentication or backend database synchronization (all studio operations run in-browser with local state).
- Replacing or breaking existing build configurations in `apps/admin-dashboard`.

---

## System Architecture

```
                                  [ Browser (localhost:3000) ]
                                                │
                                                ▼
                                    ┌───────────────────────┐
                                    │      apps/portal      │
                                    │ (Initial Eager Bundle)│
                                    └───────────┬───────────┘
                                                │
         ┌────────────────────────┬─────────────┴────────────┬────────────────────────┐
         ▼                        ▼                          ▼                        ▼
┌──────────────────┐    ┌──────────────────┐       ┌──────────────────┐     ┌──────────────────┐
│  Executive Hub   │    │Spreadsheet Studio│       │    PDF Studio    │     │ Document Studio  │
│  (libs/charting) │    │(libs/spreadsheet)│       │ (libs/pdf-studio)│     │(libs/doc-editor) │
│ - Chart.js       │    │ - ExcelJS        │       │ - PDF.js         │     │ - Markdown/Rich  │
│ - Moment.js      │    │ - Data Grid      │       │ - Canvas preview │     │ - Code format    │
│ - Lodash-es      │    │ - Formula Engine │       │ - Blob generator │     │ - Live preview   │
└──────────────────┘    └──────────────────┘       └──────────────────┘     └──────────────────┘
```

### 1. Host Application: `apps/portal`
- **Shell Layout**:
  - `HeaderComponent`: Studio branding ("DocuCraft Studio"), active document status, quick actions, export shortcuts.
  - `SidebarComponent`: Navigation links for `/overview`, `/sheets`, `/pdf`, `/documents`.
  - `MetricsBannerComponent`: Displays active route, loaded chunk diagnostics, and live module memory footprint.
- **Router Configuration**:
  - Path `/` redirects to `/overview`.
  - Path `/overview` loads the executive dashboard (eagerly bundled).
  - Path `/sheets` lazy-loads `SpreadsheetStudioComponent` (`libs/spreadsheet-studio`).
  - Path `/pdf` lazy-loads `PdfStudioComponent` (`libs/pdf-studio`).
  - Path `/documents` lazy-loads `DocumentEditorComponent` (`libs/document-editor`).

### 2. Feature Libraries (`libs/`)

#### `libs/charting` (Existing - enhanced)
- **Role**: Powers time-series metrics and KPI visualizers.
- **Packages**: `chart.js`, `moment`.
- **Exports**: `ChartEngineService`, `ExecutiveMetricsChartComponent`, `DateFormatter`.

#### `libs/reports` (Existing - enhanced)
- **Role**: Shared report definitions, data schemas, and binary utilities.
- **Packages**: `lodash-es`, `tslib`.
- **Exports**: `ReportDatasetService`, `ReportTemplate`, shared financial data models.

#### `libs/spreadsheet-studio` (New)
- **Role**: Interactive in-browser spreadsheet workspace with real `.xlsx` export.
- **Packages**: `exceljs`, `lodash`.
- **Features**:
  - Interactive grid with editable cells, rows, and headers.
  - Calculation summaries (Total revenue, Average margin, Count).
  - One-click Excel `.xlsx` file generation and download with custom styling.

#### `libs/pdf-studio` (New)
- **Role**: Document generation and PDF inspection workspace.
- **Packages**: `pdfjs-dist`.
- **Features**:
  - Interactive document builder (invoice / executive summary layout).
  - PDF binary generation via client-side canvas rendering and blob export.
  - Embedded viewer canvas powered by `pdfjs-dist` to render and inspect pages.

#### `libs/document-editor` (New)
- **Role**: Formatted report editor and markdown studio.
- **Packages**: `lodash-es`, standard browser DOM APIs.
- **Features**:
  - Dual-pane live markdown/rich document editor.
  - Formatting controls (bold, headings, tables, callouts).
  - Sanitized HTML previewer with real-time word and character counts.

---

## Data Models & State Management

- **Central Report State**: Managed in `libs/reports` via an Angular injectable service (`ReportDatasetService`) with Angular Signals (`signal`, `computed`).
- **Reactive Flow**:
  - Modifying financial line items in the Spreadsheet Studio automatically updates the KPI calculations and chart representations in Executive Overview and PDF export preview.

---

## Verification & Testing Plan

1. **Build Verification**:
   - `npm run build` in `testdata/nx-workspace`:
     - Compiles all libraries and applications without TypeScript or lint errors.
     - Emits `dist/apps/portal/stats.json` with separate chunks for `portal`, `sheets`, `pdf`, and `documents`.
2. **Local Serving Verification**:
   - `npm start` serves `apps/portal` at `http://localhost:3000`.
   - Verify:
     - Navigation between `/overview`, `/sheets`, `/pdf`, and `/documents`.
     - Spreadsheet edits and Excel `.xlsx` file download.
     - PDF generator and preview canvas rendering.
     - Chart.js visualizer rendering.
3. **Bundlecheck Tooling Verification**:
   - Run `go test ./...` from repo root:
     - Ensures `bundlecheck summary --project portal` continues to pass.
     - Ensures `bundlecheck suggest --project portal` continues to advise on heavy dependencies (`moment`, `lodash`).
     - Ensures `bundlecheck why moment --project portal` successfully traces import paths.
