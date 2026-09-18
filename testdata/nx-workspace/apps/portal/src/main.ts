import { bootstrapApplication } from '@angular/platform-browser';
import { appConfig } from './app/app.config';
import { AppComponent } from './app/app.component';

// GOTCHA 1: Accidental Entrypoint Leak
// Importing a simple date formatting function from @nx-workspace/charting
// pulls the ENTIRE Chart.js, D3, and Moment.js tree into initial main.js!
import { formatSimpleDate } from '@nx-workspace/charting';

console.log('App started at:', formatSimpleDate(new Date()));

bootstrapApplication(AppComponent, appConfig)
  .catch((err) => console.error(err));
