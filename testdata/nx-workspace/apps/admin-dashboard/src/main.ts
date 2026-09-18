import { bootstrapApplication } from '@angular/platform-browser';
import { Component } from '@angular/core';
import * as ExcelJS from 'exceljs';
import _ from 'lodash';

@Component({
  standalone: true,
  selector: 'app-admin-root',
  template: `<h1>Admin Dashboard</h1>`
})
export class AdminRootComponent {
  exportExcel() {
    const wb = new ExcelJS.Workbook();
    return wb;
  }

  processUsers(users: any[]) {
    return _.groupBy(users, 'role');
  }
}

bootstrapApplication(AdminRootComponent)
  .catch((err) => console.error(err));
