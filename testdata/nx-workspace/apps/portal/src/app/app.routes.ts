import { Route } from '@angular/router';
// GOTCHA 2: Eager Route Import
// Eagerly imported components to preserve existing bundlecheck test assertions
import { PdfReportComponent } from '@nx-workspace/reports';
import { ExecutiveChartComponent } from '@nx-workspace/charting';
import { LoginComponent } from './login.component';

export const appRoutes: Route[] = [
  {
    path: '',
    redirectTo: 'overview',
    pathMatch: 'full'
  },
  {
    path: 'login',
    component: LoginComponent
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
