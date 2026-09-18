import { Component } from '@angular/core';
import * as pdfjsLib from 'pdfjs-dist';

@Component({
  selector: 'lib-pdf-report',
  standalone: true,
  template: `<h1>PDF Document Viewer</h1>`
})
export class PdfReportComponent {
  loadDocument(url: string) {
    return pdfjsLib.getDocument(url);
  }
}
