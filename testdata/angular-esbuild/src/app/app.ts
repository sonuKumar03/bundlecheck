import { Component } from '@angular/core';
import { RouterLink, RouterOutlet } from '@angular/router';

@Component({
  selector: 'app-root',
  imports: [RouterLink, RouterOutlet],
  template: `<h1>bundlecheck smoke test</h1><a routerLink="/">Home</a> <a routerLink="/lazy">Lazy report</a><router-outlet />`,
})
export class App {}
