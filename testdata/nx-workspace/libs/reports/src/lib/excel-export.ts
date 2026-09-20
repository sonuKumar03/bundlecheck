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

export class ExcelExporter {
  exportData(rows: any[]) {
    const workbook = new ExcelJS.Workbook();
    const sheet = workbook.addWorksheet('Report');
    sheet.addRows(rows);
    return workbook.xlsx.writeBuffer();
  }
}
