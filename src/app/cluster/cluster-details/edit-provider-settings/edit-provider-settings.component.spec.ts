import {ComponentFixture, TestBed} from '@angular/core/testing';
import {BrowserModule} from '@angular/platform-browser';
import {BrowserAnimationsModule} from '@angular/platform-browser/animations';
import {Router} from '@angular/router';

import {ClusterService} from '../../../core/services';
import {SharedModule} from '../../../shared/shared.module';
import {fakeDigitaloceanCluster} from '../../../testing/fake-data/cluster.fake';
import {RouterStub} from '../../../testing/router-stubs';
import {ClusterMockService} from '../../../testing/services/cluster-mock-service';

import {AWSProviderSettingsComponent} from './aws-provider-settings/aws-provider-settings.component';
import {AzureProviderSettingsComponent} from './azure-provider-settings/azure-provider-settings.component';
import {DigitaloceanProviderSettingsComponent} from './digitalocean-provider-settings/digitalocean-provider-settings.component';
import {EditProviderSettingsComponent} from './edit-provider-settings.component';
import {GCPProviderSettingsComponent} from './gcp-provider-settings/gcp-provider-settings.component';
import {HetznerProviderSettingsComponent} from './hetzner-provider-settings/hetzner-provider-settings.component';
import {KubevirtProviderSettingsComponent} from './kubevirt-provider-settings/kubevirt-provider-settings.component';
import {OpenstackProviderSettingsComponent} from './openstack-provider-settings/openstack-provider-settings.component';
import {PacketProviderSettingsComponent} from './packet-provider-settings/packet-provider-settings.component';
import {VSphereProviderSettingsComponent} from './vsphere-provider-settings/vsphere-provider-settings.component';

const modules: any[] = [
  BrowserModule,
  BrowserAnimationsModule,
  SharedModule,
];

describe('EditProviderSettingsComponent', () => {
  let fixture: ComponentFixture<EditProviderSettingsComponent>;
  let component: EditProviderSettingsComponent;

  beforeEach(() => {
    TestBed
        .configureTestingModule({
          imports: [
            ...modules,
          ],
          declarations: [
            EditProviderSettingsComponent,
            AWSProviderSettingsComponent,
            DigitaloceanProviderSettingsComponent,
            HetznerProviderSettingsComponent,
            OpenstackProviderSettingsComponent,
            VSphereProviderSettingsComponent,
            AzureProviderSettingsComponent,
            PacketProviderSettingsComponent,
            GCPProviderSettingsComponent,
            KubevirtProviderSettingsComponent,
          ],
          providers: [
            {provide: Router, useClass: RouterStub},
            {provide: ClusterService, useClass: ClusterMockService},
          ],
        })
        .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(EditProviderSettingsComponent);
    component = fixture.componentInstance;
    component.cluster = fakeDigitaloceanCluster();
    fixture.detectChanges();
  });

  it('should create the edit provider settings cmp', () => {
    expect(component).toBeTruthy();
  });
});
