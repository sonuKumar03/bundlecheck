import { Component } from '@angular/core';
// Gotcha: CommonJS lodash import preventing tree-shaking
import _ from 'lodash';

@Component({
  selector: 'lib-modal',
  standalone: true,
  template: `<div class="modal">Modal Dialog</div>`
})
export class ModalComponent {
  cloneData(obj: any) {
    return _.cloneDeep(obj);
  }
}
