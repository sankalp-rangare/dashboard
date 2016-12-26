import { Component, OnInit } from "@angular/core";
import {ApiService} from "../api/api.service";
import {DataCenterEntity} from "../api/entitiy/DatacenterEntity";
import {ClusterNameGenerator} from "../util/name-generator.service";
import {FormBuilder, FormGroup, Validators} from "@angular/forms";


@Component({
  selector: "kubermatic-wizard",
  templateUrl: "./wizard.component.html",
  styleUrls: ["./wizard.component.scss"]
})
export class WizardComponent implements OnInit {

  public seedDataCenters: DataCenterEntity[] = [];
  public supportedNodeProviders: string[] = ["aws", "digitalocean", "bringyourown"];
  public groupedDatacenters: {[key: string]: DataCenterEntity[]} = {};

  public currentStep: number = 0;
  public stepsTitles: string[] = ["Data center", "Cloud provider", "Configuration", "Go!"];

  public selectedDC: string;
  public selectedCloud: string;
  public selectedCloudRegion: string;
  public selectedCloudConfiguration: any;
  public selectedNodeCount: number = 3;
  public acceptBringYourOwn: boolean;

  public clusterNameForm: FormGroup;
  public bringYourOwnForm: FormGroup;

  constructor(private api: ApiService, private nameGenerator: ClusterNameGenerator, private formBuilder: FormBuilder) {
  }

  ngOnInit() {
    this.api.getDataCenters().subscribe(result => {
      this.seedDataCenters = result.filter(elem => elem.seed)
        .sort((a, b) => DataCenterEntity.sortByName(a, b));

      result.forEach(elem => {
        if (!this.groupedDatacenters.hasOwnProperty(elem.spec.provider)) {
          this.groupedDatacenters[elem.spec.provider] = [];
        }

        this.groupedDatacenters[elem.spec.provider].push(elem);
      });
      // console.log(JSON.stringify(this.seedDataCenters));
    });

    this.bringYourOwnForm = this.formBuilder.group({
      pif: ["", [<any>Validators.required, <any>Validators.minLength(2), <any>Validators.maxLength(16),
        Validators.pattern("[a-z0-9-]+(:[a-z0-9-]+)?")]],
    });

    this.clusterNameForm = this.formBuilder.group({
      clustername: ["", [<any>Validators.required, <any>Validators.minLength(2), <any>Validators.maxLength(16)]],
    });
    
    this.refreshName();
  }

  public selectDC(dc: string) {
    this.selectedDC = dc;
    this.selectedCloud = null;
    this.selectedCloudRegion = null;
  }

  public selectCloud(cloud: string) {
    this.selectedCloud = cloud;
    this.selectedCloudRegion = null;
  }

  public selectCloudRegion(cloud: string) {
    this.selectedCloudRegion = cloud;
  }

  public selectCloudConfiguration(config: any) {
    this.selectedCloudConfiguration = config;
  }

  public selectNodeCount(nodeCount: number) {
    this.selectedNodeCount = nodeCount;
  }

  public gotoStep(step: number) {
    this.currentStep = step;
  }

  public canGotoStep(step: number) {
    switch (step) {
      case 0:
        return !!this.selectedDC;
      case 1:
        return !!this.selectedCloud;
      case 2:
        if (this.selectedCloud === "bringyourown") {
          return this.acceptBringYourOwn;
        } else {
          return !!this.selectedCloudRegion;
        }
      case 3:
        if (this.selectedCloud === "bringyourown") {
          return this.bringYourOwnForm.valid && this.clusterNameForm.valid;
        } else {
          return this.clusterNameForm.valid; // TODO
        }
      case 4:
        return !!this.selectedNodeCount && this.selectedNodeCount >= 0;
      default:
        return false;
    }
  }

  public stepBack() {
    this.currentStep = (this.currentStep - 1) < 0 ? 0 : (this.currentStep - 1);
  }

  public stepForward() {
    this.currentStep = (this.currentStep + 1) > this.stepsTitles.length ? 0 : (this.currentStep + 1);
  }

  public canStepBack(): boolean {
    return this.currentStep > 0;
  }

  public canStepForward(): boolean {
    return this.canGotoStep(this.currentStep);
  }

  public refreshName() {
    this.clusterNameForm.patchValue({clustername: this.nameGenerator.generateName()});
  }
}
