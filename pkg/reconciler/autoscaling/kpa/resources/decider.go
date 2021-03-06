/*
Copyright 2018 The Knative Authors

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

package resources

import (
	"context"

	"github.com/knative/pkg/logging"
	"github.com/knative/serving/pkg/apis/autoscaling/v1alpha1"
	"github.com/knative/serving/pkg/autoscaler"
	"github.com/knative/serving/pkg/reconciler/autoscaling/resources"
)

// Deciders is an interface for notifying the presence or absence of KPAs.
type Deciders interface {
	// Get accesses the Decider resource for this key, returning any errors.
	Get(ctx context.Context, namespace, name string) (*autoscaler.Decider, error)

	// Create adds a Decider resource for a given key, returning any errors.
	Create(ctx context.Context, decider *autoscaler.Decider) (*autoscaler.Decider, error)

	// Delete removes the Decider resource for a given key, returning any errors.
	Delete(ctx context.Context, namespace, name string) error

	// Watch registers a function to call when Decider change.
	Watch(watcher func(string))

	// Update update the Decider resource, return the new Decider or any errors.
	Update(ctx context.Context, decider *autoscaler.Decider) (*autoscaler.Decider, error)
}

// MakeDecider constructs a Decider resource from a PodAutoscaler taking
// into account the PA's ContainerConcurrency and the relevant
// autoscaling annotation.
func MakeDecider(ctx context.Context, pa *v1alpha1.PodAutoscaler, config *autoscaler.Config, svc string) *autoscaler.Decider {
	logger := logging.FromContext(ctx)

	target := config.TargetConcurrency(pa.Spec.ContainerConcurrency)
	if mt, ok := pa.Target(); ok {
		annotationTarget := float64(mt)
		if annotationTarget > target {
			// If the annotation target would cause the autoscaler to maintain
			// more requests per pod than the container can handle, we ignore
			// the annotation and use a containerConcurrency based target instead.
			logger.Warnf("Ignoring target of %v because it would underprovision the Revision.", annotationTarget)
		} else {
			logger.Debugf("Using target of %v", annotationTarget)
			target = annotationTarget
		}
	}
	// Look for a panic threshold percentage annotation.
	panicThresholdPercentage, ok := pa.PanicThresholdPercentage()
	if !ok {
		// Fall back on cluster config.
		panicThresholdPercentage = config.PanicThresholdPercentage
	}
	panicThreshold := target * panicThresholdPercentage / 100.0
	// TODO: remove MetricSpec when the custom metrics adapter implements Metric.
	metricSpec := resources.MakeMetric(ctx, pa, config).Spec
	return &autoscaler.Decider{
		ObjectMeta: *pa.ObjectMeta.DeepCopy(),
		Spec: autoscaler.DeciderSpec{
			TickInterval:      config.TickInterval,
			MaxScaleUpRate:    config.MaxScaleUpRate,
			TargetConcurrency: target,
			PanicThreshold:    panicThreshold,
			MetricSpec:        metricSpec,
			ServiceName:       svc,
		},
	}
}
