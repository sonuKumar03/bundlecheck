import * as ExcelJS from 'exceljs';

export class ExcelExporter {
  exportData(rows: any[]) {
    const workbook = new ExcelJS.Workbook();
    const sheet = workbook.addWorksheet('Report');
    sheet.addRows(rows);
    return workbook.xlsx.writeBuffer();
  }
}
