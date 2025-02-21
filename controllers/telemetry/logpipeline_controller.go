package telemetry

/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/predicate"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	logpipelinefluentbit "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/fluentbit"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/otel"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

var (
	fluentBitCPULimit      = resource.MustParse("1")
	fluentBitMemoryLimit   = resource.MustParse("1Gi")
	fluentBitCPURequest    = resource.MustParse("100m")
	fluentBitMemoryRequest = resource.MustParse("50Mi")
)

// LogPipelineController reconciles a LogPipeline object
type LogPipelineController struct {
	client.Client

	reconcileTriggerChan <-chan event.GenericEvent
	reconciler           *logpipeline.Reconciler
}

type LogPipelineControllerConfig struct {
	ExporterImage      string
	FluentBitImage     string
	PriorityClassName  string
	RestConfig         *rest.Config
	SelfMonitorName    string
	TelemetryNamespace string
}

func NewLogPipelineController(client client.Client, reconcileTriggerChan <-chan event.GenericEvent, config LogPipelineControllerConfig) (*LogPipelineController, error) {
	flowHealthProber, err := prober.NewLogPipelineProber(types.NamespacedName{Name: config.SelfMonitorName, Namespace: config.TelemetryNamespace})
	if err != nil {
		return nil, err
	}

	fluentbitConfig := logpipelinefluentbit.Config{
		SectionsConfigMap:     types.NamespacedName{Name: "telemetry-fluent-bit-sections", Namespace: config.TelemetryNamespace},
		FilesConfigMap:        types.NamespacedName{Name: "telemetry-fluent-bit-files", Namespace: config.TelemetryNamespace},
		LuaConfigMap:          types.NamespacedName{Name: "telemetry-fluent-bit-luascripts", Namespace: config.TelemetryNamespace},
		ParsersConfigMap:      types.NamespacedName{Name: "telemetry-fluent-bit-parsers", Namespace: config.TelemetryNamespace},
		EnvSecret:             types.NamespacedName{Name: "telemetry-fluent-bit-env", Namespace: config.TelemetryNamespace},
		OutputTLSConfigSecret: types.NamespacedName{Name: "telemetry-fluent-bit-output-tls-config", Namespace: config.TelemetryNamespace},
		DaemonSet:             types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: config.TelemetryNamespace},
		PipelineDefaults: builder.PipelineDefaults{
			InputTag:          "tele",
			MemoryBufferLimit: "10M",
			StorageType:       "filesystem",
			FsBufferLimit:     "1G",
		},
		DaemonSetConfig: fluentbit.DaemonSetConfig{
			FluentBitImage:    config.FluentBitImage,
			ExporterImage:     config.ExporterImage,
			PriorityClassName: config.PriorityClassName,
			CPULimit:          fluentBitCPULimit,
			MemoryLimit:       fluentBitMemoryLimit,
			CPURequest:        fluentBitCPURequest,
			MemoryRequest:     fluentBitMemoryRequest,
		},
	}

	pipelineValidator := &logpipelinefluentbit.Validator{
		EndpointValidator:  &endpoint.Validator{Client: client},
		TLSCertValidator:   tlscert.New(client),
		SecretRefValidator: &secretref.Validator{Client: client},
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config.RestConfig)
	if err != nil {
		return nil, err
	}

	fbReconciler := logpipelinefluentbit.New(client, fluentbitConfig, &workloadstatus.DaemonSetProber{Client: client}, flowHealthProber, istiostatus.NewChecker(discoveryClient), pipelineValidator, &conditions.ErrorToMessageConverter{})
	otelReconciler := otel.New(client, &conditions.ErrorToMessageConverter{})

	reconciler := logpipeline.New(
		client,
		overrides.New(client, overrides.HandlerConfig{SystemNamespace: config.TelemetryNamespace}),
		fbReconciler,
		otelReconciler,
	)

	return &LogPipelineController{
		Client:               client,
		reconcileTriggerChan: reconcileTriggerChan,
		reconciler:           reconciler,
	}, nil
}

func (r *LogPipelineController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

func (r *LogPipelineController) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).For(&telemetryv1alpha1.LogPipeline{})

	b.WatchesRawSource(
		source.Channel(r.reconcileTriggerChan, &handler.EnqueueRequestForObject{}),
	)

	ownedResourceTypesToWatch := []client.Object{
		&appsv1.DaemonSet{},
		&corev1.ConfigMap{},
		&corev1.Pod{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
	}

	for _, resource := range ownedResourceTypesToWatch {
		b = b.Watches(
			resource,
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(),
				mgr.GetRESTMapper(),
				&telemetryv1alpha1.LogPipeline{},
			),
			ctrlbuilder.WithPredicates(predicate.OwnedResourceChanged()),
		)
	}

	return b.Complete(r)
}
