import {Component, Input, OnDestroy, OnInit} from '@angular/core';
import {FormArray, FormControl, FormGroup, Validators} from '@angular/forms';
import {Subject} from 'rxjs';
import {takeUntil} from 'rxjs/operators';
import {WizardService} from '../../core/services';
import {NodeDataService} from '../../core/services/node-data/node-data.service';
import {CloudSpec} from '../../shared/entity/ClusterEntity';
import {NodeInstanceFlavors} from '../../shared/model/NodeProviderConstants';
import {NodeData, NodeProviderData} from '../../shared/model/NodeSpecChange';

@Component({
  selector: 'kubermatic-gcp-node-data',
  templateUrl: './gcp-node-data.component.html',
  styleUrls: ['./gcp-node-data.component.scss'],
})

export class GCPNodeDataComponent implements OnInit, OnDestroy {
  @Input() cloudSpec: CloudSpec;
  @Input() nodeData: NodeData;
  diskTypes: string[] = NodeInstanceFlavors.GCP.DiskTypes;
  machineTypes: string[] = NodeInstanceFlavors.GCP.MachineTypes;
  gcpNodeForm: FormGroup;
  labels: FormArray;
  hideOptional = true;
  private _unsubscribe: Subject<any> = new Subject();

  constructor(private addNodeService: NodeDataService, private wizardService: WizardService) {}

  ngOnInit(): void {
    const labelList = new FormArray([]);
    for (const i in this.nodeData.spec.cloud.gcp.labels) {
      if (this.nodeData.spec.cloud.gcp.labels.hasOwnProperty(i)) {
        labelList.push(new FormGroup({
          key: new FormControl(i),
          value: new FormControl(this.nodeData.spec.cloud.gcp.labels[i]),
        }));
      }
    }

    this.gcpNodeForm = new FormGroup({
      diskSize: new FormControl(this.nodeData.spec.cloud.gcp.diskSize, Validators.required),
      diskType: new FormControl(this.nodeData.spec.cloud.gcp.diskType, Validators.required),
      machineType: new FormControl(this.nodeData.spec.cloud.gcp.machineType, Validators.required),
      zone: new FormControl(this.nodeData.spec.cloud.gcp.zone, Validators.required),
      preemptible: new FormControl(this.nodeData.spec.cloud.gcp.preemptible),
      tags: new FormControl(this.nodeData.spec.cloud.gcp.tags.toString().replace(/\,/g, ', ')),
      labels: labelList,
    });

    if (this.nodeData.spec.cloud.gcp.diskType === '') {
      this.gcpNodeForm.controls.diskType.setValue(this.diskTypes[0]);
    }

    if (this.nodeData.spec.cloud.gcp.machineType === '') {
      this.gcpNodeForm.controls.machineType.setValue(this.machineTypes[0]);
    }

    this.gcpNodeForm.valueChanges.pipe(takeUntil(this._unsubscribe)).subscribe(() => {
      this.addNodeService.changeNodeProviderData(this.getNodeProviderData());
    });

    this.wizardService.clusterSettingsFormViewChanged$.pipe(takeUntil(this._unsubscribe)).subscribe((data) => {
      this.hideOptional = data.hideOptional;
    });

    this.addNodeService.changeNodeProviderData(this.getNodeProviderData());
  }

  getNodeProviderData(): NodeProviderData {
    const labelsMap = {};
    for (const i in this.gcpNodeForm.controls.labels.value) {
      if (this.gcpNodeForm.controls.labels.value[i].key !== '' &&
          this.gcpNodeForm.controls.labels.value[i].value !== '') {
        labelsMap[this.gcpNodeForm.controls.labels.value[i].key] = this.gcpNodeForm.controls.labels.value[i].value;
      }
    }

    let gcpTags: string[] = [];
    if ((this.gcpNodeForm.controls.tags.value).length > 0) {
      gcpTags = (this.gcpNodeForm.controls.tags.value).split(',').map(tag => tag.trim());
      gcpTags.map(tag => tag.trim());
    }

    return {
      spec: {
        gcp: {
          diskSize: this.gcpNodeForm.controls.diskSize.value,
          diskType: this.gcpNodeForm.controls.diskType.value,
          machineType: this.gcpNodeForm.controls.machineType.value,
          preemptible: this.gcpNodeForm.controls.preemptible.value,
          zone: this.gcpNodeForm.controls.zone.value,
          labels: labelsMap,
          tags: gcpTags,
        },
      },
      valid: this.gcpNodeForm.valid,
    };
  }

  getLabelForm(form): any {
    return form.get('labels').controls;
  }

  addLabel(): void {
    this.labels = this.gcpNodeForm.get('labels') as FormArray;
    this.labels.push(new FormGroup({
      key: new FormControl(''),
      value: new FormControl(''),
    }));
  }

  deleteLabel(index: number): void {
    const arrayControl = this.gcpNodeForm.get('labels') as FormArray;
    arrayControl.removeAt(index);
  }

  ngOnDestroy(): void {
    this._unsubscribe.next();
    this._unsubscribe.complete();
  }
}
