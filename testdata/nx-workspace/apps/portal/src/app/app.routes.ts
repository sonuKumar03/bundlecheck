import { Route } from '@angular/router';
// GOTCHA 2: Eager Route Import
// Importing PdfReportComponent eagerly instead of using loadComponent: () => import(...)
// forces pdfjs-dist and exceljs directly into the initial bundle.
import { PdfReportComponent } from '@nx-workspace/reports';

export const appRoutes: Route[] = [
  {
    path: 'reports',
    component: PdfReportComponent
  },
  {
    path: 'charts',
    // Properly lazy loaded route
    loadComponent: () => import('@nx-workspace/charting').then(m => m.ChartEngine as any)
  }
];
