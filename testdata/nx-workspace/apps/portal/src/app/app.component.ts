import { Component } from '@angular/core';
import { RouterModule } from '@angular/router';
// GOTCHA 3: CommonJS lodash import
import cloneDeep from 'lodash/cloneDeep';

@Component({
  standalone: true,
  imports: [RouterModule],
  selector: 'app-root',
  template: `
    <header>
      <h1>Portal Application</h1>
      <nav>
        <a routerLink="/reports">Reports</a>
        <a routerLink="/charts">Charts</a>
      </nav>
    </header>
    <router-outlet></router-outlet>
  `
})
export class AppComponent {
  title = 'portal';

  copyConfig(cfg: any) {
    return cloneDeep(cfg);
  }
}
