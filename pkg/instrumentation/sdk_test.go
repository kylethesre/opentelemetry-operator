// Copyright The OpenTelemetry Authors
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

package instrumentation

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
)

func TestSDKInjection(t *testing.T) {
	tests := []struct {
		name     string
		inst     v1alpha1.Instrumentation
		pod      corev1.Pod
		expected corev1.Pod
	}{
		{
			name: "SDK env vars not defined",
			inst: v1alpha1.Instrumentation{
				Spec: v1alpha1.InstrumentationSpec{
					Exporter: v1alpha1.Exporter{
						Endpoint: "https://collector:4317",
					},
					Propagators: []v1alpha1.Propagator{"b3", "jaeger"},
					Sampler: v1alpha1.Sampler{
						Type:     "parentbased_traceidratio",
						Argument: "0.25",
					},
				},
			},
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "project1",
					Name:      "app",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "application-name",
						},
					},
				},
			},
			expected: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "project1",
					Name:      "app",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "application-name",
							Env: []corev1.EnvVar{
								{
									Name:  "OTEL_SERVICE_NAME",
									Value: "application-name",
								},
								{
									Name:  "OTEL_EXPORTER_OTLP_ENDPOINT",
									Value: "https://collector:4317",
								},
								{
									Name:  "OTEL_RESOURCE_ATTRIBUTES",
									Value: "k8s.container.name=application-name,k8s.namespace.name=project1,k8s.pod.name=app",
								},
								{
									Name:  "OTEL_PROPAGATORS",
									Value: "b3,jaeger",
								},
								{
									Name:  "OTEL_TRACES_SAMPLER",
									Value: "parentbased_traceidratio",
								},
								{
									Name:  "OTEL_TRACES_SAMPLER_ARG",
									Value: "0.25",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "SDK env vars defined",
			inst: v1alpha1.Instrumentation{
				Spec: v1alpha1.InstrumentationSpec{
					Exporter: v1alpha1.Exporter{
						Endpoint: "https://collector:4317",
					},
					ResourceAttributes: map[string]string{
						"fromcr": "val",
					},
					Propagators: []v1alpha1.Propagator{"jaeger"},
					Sampler: v1alpha1.Sampler{
						Type:     "parentbased_traceidratio",
						Argument: "0.25",
					},
				},
			},
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "project1",
					Name:      "app",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:  "OTEL_SERVICE_NAME",
									Value: "explicitly_set",
								},
								{
									Name:  "OTEL_EXPORTER_OTLP_ENDPOINT",
									Value: "explicitly_set",
								},
								{
									Name:  "OTEL_RESOURCE_ATTRIBUTES",
									Value: "foo=bar,k8s.container.name=other,",
								},
								{
									Name:  "OTEL_PROPAGATORS",
									Value: "b3",
								},
								{
									Name:  "OTEL_TRACES_SAMPLER",
									Value: "always_on",
								},
							},
						},
					},
				},
			},
			expected: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "project1",
					Name:      "app",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:  "OTEL_SERVICE_NAME",
									Value: "explicitly_set",
								},
								{
									Name:  "OTEL_EXPORTER_OTLP_ENDPOINT",
									Value: "explicitly_set",
								},
								{
									Name:  "OTEL_RESOURCE_ATTRIBUTES",
									Value: "foo=bar,k8s.container.name=other,fromcr=val,k8s.namespace.name=project1,k8s.pod.name=app",
								},
								{
									Name:  "OTEL_PROPAGATORS",
									Value: "b3",
								},
								{
									Name:  "OTEL_TRACES_SAMPLER",
									Value: "always_on",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pod := injectCommonSDKConfig(test.inst, corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: test.pod.Namespace}}, test.pod)
			assert.Equal(t, test.expected, pod)
		})
	}
}

func TestInjectJava(t *testing.T) {
	inst := v1alpha1.Instrumentation{
		Spec: v1alpha1.InstrumentationSpec{
			Java: v1alpha1.JavaSpec{
				Image: "img:1",
			},
			Exporter: v1alpha1.Exporter{
				Endpoint: "https://collector:4317",
			},
		},
	}
	insts := languageInstrumentations{
		Java: &inst,
	}
	pod := inject(logr.Discard(), insts,
		corev1.Namespace{},
		corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "app",
					},
				},
			},
		})
	assert.Equal(t, corev1.Pod{
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: volumeName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:    initContainerName,
					Image:   "img:1",
					Command: []string{"cp", "/javaagent.jar", "/otel-auto-instrumentation/javaagent.jar"},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      volumeName,
						MountPath: "/otel-auto-instrumentation",
					}},
				},
			},
			Containers: []corev1.Container{
				{
					Name: "app",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      volumeName,
							MountPath: "/otel-auto-instrumentation",
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "OTEL_SERVICE_NAME",
							Value: "app",
						},
						{
							Name:  "OTEL_EXPORTER_OTLP_ENDPOINT",
							Value: "https://collector:4317",
						},
						{
							Name:  "OTEL_RESOURCE_ATTRIBUTES",
							Value: "k8s.container.name=app,k8s.namespace.name=",
						},
						{
							Name:  "JAVA_TOOL_OPTIONS",
							Value: javaJVMArgument,
						},
					},
				},
			},
		},
	}, pod)
}

func TestInjectNodeJS(t *testing.T) {
	inst := v1alpha1.Instrumentation{
		Spec: v1alpha1.InstrumentationSpec{
			NodeJS: v1alpha1.NodeJSSpec{
				Image: "img:1",
			},
			Exporter: v1alpha1.Exporter{
				Endpoint: "https://collector:4318",
			},
		},
	}
	insts := languageInstrumentations{
		NodeJS: &inst,
	}
	pod := inject(logr.Discard(), insts,
		corev1.Namespace{},
		corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "app",
					},
				},
			},
		})
	assert.Equal(t, corev1.Pod{
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: volumeName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:    initContainerName,
					Image:   "img:1",
					Command: []string{"cp", "-a", "/autoinstrumentation/.", "/otel-auto-instrumentation/"},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      volumeName,
						MountPath: "/otel-auto-instrumentation",
					}},
				},
			},
			Containers: []corev1.Container{
				{
					Name: "app",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      volumeName,
							MountPath: "/otel-auto-instrumentation",
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "OTEL_SERVICE_NAME",
							Value: "app",
						},
						{
							Name:  "OTEL_EXPORTER_OTLP_ENDPOINT",
							Value: "https://collector:4318",
						},
						{
							Name:  "OTEL_RESOURCE_ATTRIBUTES",
							Value: "k8s.container.name=app,k8s.namespace.name=",
						},
						{
							Name:  "NODE_OPTIONS",
							Value: nodeRequireArgument,
						},
					},
				},
			},
		},
	}, pod)
}
