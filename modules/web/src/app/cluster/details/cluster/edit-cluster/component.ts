// Copyright 2020 The Kubermatic Kubernetes Platform contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import {Component, Input, OnDestroy, OnInit} from '@angular/core';
import {FormBuilder, FormControl, FormGroup, Validators} from '@angular/forms';
import {MatDialogRef} from '@angular/material/dialog';
import {ClusterService} from '@core/services/cluster';
import {DatacenterService} from '@core/services/datacenter';
import {NotificationService} from '@core/services/notification';
import {SettingsService} from '@core/services/settings';
import {
  AuditPolicyPreset,
  Cluster,
  ClusterPatch,
  CNIPlugin,
  ContainerRuntime,
  END_OF_DOCKER_SUPPORT_VERSION,
  EventRateLimitConfig,
  ExposeStrategy,
  NetworkRanges,
  ProviderSettingsPatch,
  ProxyMode,
} from '@shared/entity/cluster';
import {ResourceType} from '@shared/entity/common';
import {Datacenter, SeedSettings} from '@shared/entity/datacenter';
import {AdminSettings} from '@shared/entity/settings';
import {AdmissionPlugin, AdmissionPluginUtils} from '@shared/utils/admission-plugin';
import {AsyncValidators} from '@shared/validators/async.validators';
import _ from 'lodash';
import {Observable, Subject} from 'rxjs';
import {startWith, switchMap, take, takeUntil, tap} from 'rxjs/operators';
import * as semver from 'semver';
import {FeatureGateService} from '@core/services/feature-gate';
import {
  CLUSTER_DEFAULT_NODE_SELECTOR_HINT,
  CLUSTER_DEFAULT_NODE_SELECTOR_NAMESPACE,
  CLUSTER_DEFAULT_NODE_SELECTOR_TOOLTIP,
  handleClusterDefaultNodeSelector,
} from '@shared/utils/cluster';
import {KeyValueEntry} from '@shared/types/common';
import {IPV4_IPV6_CIDR_PATTERN} from '@shared/validators/others';

enum Controls {
  Name = 'name',
  ContainerRuntime = 'containerRuntime',
  AuditLogging = 'auditLogging',
  AuditPolicyPreset = 'auditPolicyPreset',
  Labels = 'labels',
  AdmissionPlugins = 'admissionPlugins',
  PodNodeSelectorAdmissionPluginConfig = 'podNodeSelectorAdmissionPluginConfig',
  EventRateLimitConfig = 'eventRateLimitConfig',
  OPAIntegration = 'opaIntegration',
  Konnectivity = 'konnectivity',
  MLALogging = 'loggingEnabled',
  MLAMonitoring = 'monitoringEnabled',
  OperatingSystemManager = 'enableOperatingSystemManager',
  KubernetesDashboardEnabled = 'kubernetesDashboardEnabled',
  APIServerAllowedIPRanges = 'apiServerAllowedIPRanges',
}

@Component({
  selector: 'km-edit-cluster',
  templateUrl: './template.html',
  styleUrls: ['./style.scss'],
})
export class EditClusterComponent implements OnInit, OnDestroy {
  @Input() cluster: Cluster;
  @Input() projectID: string;
  datacenter: Datacenter;
  containerRuntime = ContainerRuntime;
  admissionPlugin = AdmissionPlugin;
  form: FormGroup;
  labels: Record<string, string>;
  initialClusterDefaultNodeSelectorKey: string;
  podNodeSelectorAdmissionPluginConfig: Record<string, string>;
  eventRateLimitConfig: EventRateLimitConfig;
  admissionPlugins: string[] = [];
  isKonnectivityEnabled = false;
  providerSettingsPatch: ProviderSettingsPatch = {
    isValid: true,
    cloudSpecPatch: {},
  };
  asyncLabelValidators = [AsyncValidators.RestrictedLabelKeyName(ResourceType.Cluster)];
  clusterDefaultNodeSelectorNamespace: KeyValueEntry;
  apiServerAllowedIPRanges: string[] = [];
  readonly CLUSTER_DEFAULT_NODE_SELECTOR_NAMESPACE = CLUSTER_DEFAULT_NODE_SELECTOR_NAMESPACE;
  readonly CLUSTER_DEFAULT_NODE_SELECTOR_TOOLTIP = CLUSTER_DEFAULT_NODE_SELECTOR_TOOLTIP;
  readonly CLUSTER_DEFAULT_NODE_SELECTOR_HINT = CLUSTER_DEFAULT_NODE_SELECTOR_HINT;
  readonly Controls = Controls;
  readonly AuditPolicyPreset = AuditPolicyPreset;
  readonly ipv4AndIPv6CidrRegex = IPV4_IPV6_CIDR_PATTERN;
  private readonly _nameMinLen = 3;
  private _settings: AdminSettings;
  private _seedSettings: SeedSettings;
  private _unsubscribe = new Subject<void>();

  constructor(
    private readonly _builder: FormBuilder,
    private readonly _clusterService: ClusterService,
    private readonly _datacenterService: DatacenterService,
    private readonly _matDialogRef: MatDialogRef<EditClusterComponent>,
    private readonly _notificationService: NotificationService,
    private readonly _settingsService: SettingsService,
    private readonly _featureGatesService: FeatureGateService
  ) {}

  ngOnInit(): void {
    this._featureGatesService.featureGates
      .pipe(takeUntil(this._unsubscribe))
      .subscribe(featureGates => (this.isKonnectivityEnabled = !!featureGates?.konnectivityService));

    this.labels = _.cloneDeep(this.cluster.labels) as Record<string, string>;
    this.podNodeSelectorAdmissionPluginConfig = _.cloneDeep(
      this.cluster.spec.podNodeSelectorAdmissionPluginConfig
    ) as Record<string, string>;
    this.eventRateLimitConfig = _.cloneDeep(this.cluster.spec.eventRateLimitConfig);

    this.form = this._builder.group({
      [Controls.Name]: new FormControl(this.cluster.name, [
        Validators.required,
        Validators.minLength(this._nameMinLen),
      ]),
      [Controls.ContainerRuntime]: new FormControl(this.cluster.spec.containerRuntime || ContainerRuntime.Containerd, [
        Validators.required,
      ]),
      [Controls.AuditLogging]: new FormControl(
        !!this.cluster.spec.auditLogging && this.cluster.spec.auditLogging.enabled
      ),
      [Controls.AuditPolicyPreset]: new FormControl(this._getAuditPolicyPresetInitialState()),
      [Controls.OPAIntegration]: new FormControl(
        !!this.cluster.spec.opaIntegration && this.cluster.spec.opaIntegration.enabled
      ),
      [Controls.Konnectivity]: new FormControl({
        value: !!this.cluster.spec.clusterNetwork?.konnectivityEnabled,
        disabled:
          this.cluster.spec.clusterNetwork?.proxyMode === ProxyMode.ebpf &&
          this.cluster.spec.cniPlugin?.type === CNIPlugin.Cilium,
      }),
      [Controls.MLALogging]: new FormControl(!!this.cluster.spec.mla && this.cluster.spec.mla.loggingEnabled),
      [Controls.MLAMonitoring]: new FormControl(!!this.cluster.spec.mla && this.cluster.spec.mla.monitoringEnabled),
      [Controls.OperatingSystemManager]: new FormControl(this.cluster.spec.enableOperatingSystemManager),
      [Controls.KubernetesDashboardEnabled]: new FormControl(!!this.cluster.spec.kubernetesDashboard?.enabled),
      [Controls.AdmissionPlugins]: new FormControl(this.cluster.spec.admissionPlugins),
      [Controls.PodNodeSelectorAdmissionPluginConfig]: new FormControl(''),
      [Controls.EventRateLimitConfig]: new FormControl(),
      [Controls.Labels]: new FormControl(null),
      [Controls.APIServerAllowedIPRanges]: new FormControl(this.cluster.spec.apiServerAllowedIPRanges),
    });

    this._settingsService.adminSettings.pipe(take(1)).subscribe(settings => {
      this._settings = settings;

      if (this._settings.opaOptions.enabled) {
        this.form.get(Controls.OPAIntegration).setValue(true);
      }
      if (this._settings.opaOptions.enforced) {
        this.form.get(Controls.OPAIntegration).disable();
      }
      this.form.updateValueAndValidity();

      this._enforce(Controls.MLALogging, this._settings.mlaOptions.loggingEnforced);
      this._enforce(Controls.MLAMonitoring, this._settings.mlaOptions.monitoringEnforced);
    });

    this.form
      .get(Controls.ContainerRuntime)
      .valueChanges.pipe(startWith(this.form.get(Controls.ContainerRuntime).value), takeUntil(this._unsubscribe))
      .subscribe(containerRuntime => {
        if (
          semver.valid(this.cluster.spec.version) &&
          semver.gte(this.cluster.spec.version, END_OF_DOCKER_SUPPORT_VERSION) &&
          containerRuntime === ContainerRuntime.Docker
        ) {
          this.form.get(Controls.ContainerRuntime).setErrors({dockerVersionCompatibility: true});
        } else {
          this.form.get(Controls.ContainerRuntime).setErrors(null);
        }
      });

    this._clusterService.providerSettingsPatchChanges$
      .pipe(takeUntil(this._unsubscribe))
      .subscribe(patch => (this.providerSettingsPatch = patch));

    this._datacenterService
      .getDatacenter(this.cluster.spec.cloud.dc)
      .pipe(tap(datacenter => (this.datacenter = datacenter)))
      .pipe(switchMap(_ => this._datacenterService.seedSettings(this.datacenter.spec.seed)))
      .pipe(takeUntil(this._unsubscribe))
      .subscribe(seedSettings => (this._seedSettings = seedSettings));

    this._clusterService
      .getAdmissionPlugins(this.cluster.spec.version)
      .pipe(takeUntil(this._unsubscribe))
      .subscribe(plugins => (this.admissionPlugins = plugins));

    this.checkForLegacyAdmissionPlugins();

    const [initialClusterDefaultNodeSelectorKey] =
      this.podNodeSelectorAdmissionPluginConfig?.[this.CLUSTER_DEFAULT_NODE_SELECTOR_NAMESPACE]?.split('=') ?? [];

    if (initialClusterDefaultNodeSelectorKey) {
      this.initialClusterDefaultNodeSelectorKey = initialClusterDefaultNodeSelectorKey;
      this._handleClusterDefaultNodeSelector(this.podNodeSelectorAdmissionPluginConfig);
    }
  }

  private _getAuditPolicyPresetInitialState(): AuditPolicyPreset | '' {
    if (!this.cluster.spec.auditLogging) {
      return '';
    }

    return this.cluster.spec.auditLogging.policyPreset
      ? this.cluster.spec.auditLogging.policyPreset
      : AuditPolicyPreset.Custom;
  }

  onLabelsChange(labels: Record<string, string>): void {
    const [key, value] = this.clusterDefaultNodeSelectorNamespace ?? [];

    if (key && value) {
      labels = {...labels, [key]: value};
    }

    if (!Object.hasOwnProperty.call(labels, this.initialClusterDefaultNodeSelectorKey)) {
      labels = {...labels, [this.initialClusterDefaultNodeSelectorKey]: null};
    }

    this.labels = labels;
  }

  onPodNodeSelectorAdmissionPluginConfigChange(config: Record<string, string>): void {
    this.podNodeSelectorAdmissionPluginConfig = config;

    this._handleClusterDefaultNodeSelector(config);
  }

  onAPIServerAllowIPRangeChange(ips: string[]): void {
    this.apiServerAllowedIPRanges = ips;
  }

  isExposeStrategyLoadBalancer(): boolean {
    return this.cluster.spec.exposeStrategy === ExposeStrategy.loadbalancer;
  }

  checkForLegacyAdmissionPlugins(): void {
    if (this.cluster.spec.usePodNodeSelectorAdmissionPlugin) {
      const value = AdmissionPluginUtils.updateSelectedPluginArray(
        this.form.get(Controls.AdmissionPlugins),
        AdmissionPlugin.PodNodeSelector
      );
      this.form.get(Controls.AdmissionPlugins).setValue(value);
    }

    if (this.cluster.spec.usePodSecurityPolicyAdmissionPlugin) {
      const value = AdmissionPluginUtils.updateSelectedPluginArray(
        this.form.get(Controls.AdmissionPlugins),
        AdmissionPlugin.PodSecurityPolicy
      );
      this.form.get(Controls.AdmissionPlugins).setValue(value);
    }

    if (this.cluster.spec.useEventRateLimitAdmissionPlugin) {
      const value = AdmissionPluginUtils.updateSelectedPluginArray(
        this.form.get(Controls.AdmissionPlugins),
        AdmissionPlugin.EventRateLimit
      );
      this.form.get(Controls.AdmissionPlugins).setValue(value);
    }

    this.checkEnforcedFieldsState();
  }

  checkEnforcedFieldsState(): void {
    if (this.datacenter.spec.enforceAuditLogging) {
      this.form.get(Controls.AuditLogging).setValue(true);
      this.form.get(Controls.AuditLogging).disable();
    }

    if (this.datacenter.spec.enforcePodSecurityPolicy) {
      const value = AdmissionPluginUtils.updateSelectedPluginArray(
        this.form.get(Controls.AdmissionPlugins),
        AdmissionPlugin.PodSecurityPolicy
      );
      this.form.get(Controls.AdmissionPlugins).setValue(value);
    }
  }

  getPluginName(name: string): string {
    return AdmissionPluginUtils.getPluginName(name);
  }

  isPluginEnabled(name: string): boolean {
    return AdmissionPluginUtils.isPluginEnabled(this.form.get(Controls.AdmissionPlugins), name);
  }

  isMLAEnabled(): boolean {
    return this._seedSettings?.mla?.user_cluster_mla_enabled;
  }

  isPodSecurityPolicyEnforced(): boolean {
    return AdmissionPluginUtils.isPodSecurityPolicyEnforced(this.datacenter);
  }

  isEnforced(control: Controls): boolean {
    switch (control) {
      case Controls.OPAIntegration:
        return !!this._settings?.opaOptions?.enforced;
      case Controls.MLALogging:
        return !!this._settings?.mlaOptions?.loggingEnforced;
      case Controls.MLAMonitoring:
        return !!this._settings?.mlaOptions?.monitoringEnforced;
      default:
        return false;
    }
  }

  private _enforce(control: Controls, isEnforced: boolean): void {
    if (isEnforced) {
      this.form.get(control).disable();
    }
  }

  getObservable(): Observable<Cluster> {
    const patch: ClusterPatch = {
      name: this.form.get(Controls.Name).value,
      labels: this.labels,
      spec: {
        cloud: this.providerSettingsPatch.cloudSpecPatch,
        auditLogging: {
          enabled: this.form.get(Controls.AuditLogging).value,
          policyPreset: this.form.get(Controls.AuditPolicyPreset).value,
        },
        opaIntegration: {
          enabled: this.form.get(Controls.OPAIntegration).value,
        },
        clusterNetwork: {
          konnectivityEnabled: this.form.get(Controls.Konnectivity).value,
        },
        kubernetesDashboard: {
          enabled: this.form.get(Controls.KubernetesDashboardEnabled).value,
        },
        enableOperatingSystemManager: this.form.get(Controls.OperatingSystemManager).value,
        mla: {
          loggingEnabled: this.form.get(Controls.MLALogging).value,
          monitoringEnabled: this.form.get(Controls.MLAMonitoring).value,
        },
        usePodNodeSelectorAdmissionPlugin: null,
        usePodSecurityPolicyAdmissionPlugin: null,
        useEventRateLimitAdmissionPlugin: null,
        eventRateLimitConfig: null,
        admissionPlugins: this.form.get(Controls.AdmissionPlugins).value,
        podNodeSelectorAdmissionPluginConfig: this.podNodeSelectorAdmissionPluginConfig,
        containerRuntime: this.form.get(Controls.ContainerRuntime).value,
      },
    };

    if (this.isPluginEnabled(this.admissionPlugin.EventRateLimit)) {
      patch.spec.eventRateLimitConfig = {
        namespace: this.form.get(Controls.EventRateLimitConfig).value,
      };
    }

    if (this.cluster.spec.exposeStrategy === ExposeStrategy.loadbalancer) {
      patch.spec.apiServerAllowedIPRanges = this.getAPIServerAllowedIPRange();
    }

    return this._clusterService.patch(this.projectID, this.cluster.id, patch).pipe(take(1));
  }

  onNext(cluster: Cluster): void {
    this._matDialogRef.close(cluster);
    this._clusterService.onClusterUpdate.next();
    this._notificationService.success(`Updated the ${this.cluster.name} cluster`);
  }

  private _handleClusterDefaultNodeSelector(config: Record<string, string>): void {
    handleClusterDefaultNodeSelector(
      this.labels ?? {},
      config,
      this.clusterDefaultNodeSelectorNamespace,
      (entry, labels): void => {
        this.clusterDefaultNodeSelectorNamespace = entry;
        this.onLabelsChange(labels);
      }
    );
  }

  private getAPIServerAllowedIPRange(): NetworkRanges {
    let apiServerAllowedIPRange = null;

    if (this.cluster.spec.exposeStrategy === ExposeStrategy.loadbalancer) {
      apiServerAllowedIPRange = this.form.get(Controls.APIServerAllowedIPRanges).value?.tags;
    }
    return apiServerAllowedIPRange === null ? null : {cidrBlocks: apiServerAllowedIPRange};
  }

  ngOnDestroy(): void {
    this._unsubscribe.next();
    this._unsubscribe.complete();
  }
}
