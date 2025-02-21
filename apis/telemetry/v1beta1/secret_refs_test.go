package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLogPipeline_GetSecretRefs(t *testing.T) {
	tests := []struct {
		name     string
		given    LogPipeline
		expected []SecretKeyRef
	}{
		{
			name: "only variables",
			given: LogPipeline{
				Spec: LogPipelineSpec{
					Variables: []LogPipelineVariableRef{
						{
							Name: "password-1",
							ValueFrom: ValueFromSource{
								SecretKeyRef: &SecretKeyRef{Name: "secret-1", Key: "password"},
							},
						},
						{
							Name: "password-2",
							ValueFrom: ValueFromSource{
								SecretKeyRef: &SecretKeyRef{Name: "secret-2", Key: "password"},
							},
						},
					},
				},
			},

			expected: []SecretKeyRef{
				{Name: "secret-1", Key: "password"},
				{Name: "secret-2", Key: "password"},
			},
		},
		{
			name: "http output secret refs",
			given: LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cls",
				},
				Spec: LogPipelineSpec{
					Output: LogPipelineOutput{
						HTTP: &LogPipelineHTTPOutput{
							Host: ValueType{
								ValueFrom: &ValueFromSource{
									SecretKeyRef: &SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "host",
									},
								},
							},
							User: ValueType{
								ValueFrom: &ValueFromSource{
									SecretKeyRef: &SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "user",
									},
								},
							},
							Password: ValueType{
								ValueFrom: &ValueFromSource{
									SecretKeyRef: &SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "password",
									},
								},
							},
						},
					},
				},
			},
			expected: []SecretKeyRef{
				{Name: "creds", Namespace: "default", Key: "host"},
				{Name: "creds", Namespace: "default", Key: "user"},
				{Name: "creds", Namespace: "default", Key: "password"},
			},
		},
		{
			name: "http output secret refs (with missing fields)",
			given: LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cls",
				},
				Spec: LogPipelineSpec{
					Output: LogPipelineOutput{
						HTTP: &LogPipelineHTTPOutput{
							Host: ValueType{
								ValueFrom: &ValueFromSource{
									SecretKeyRef: &SecretKeyRef{
										Name: "creds", Namespace: "default",
									},
								},
							},
							User: ValueType{
								ValueFrom: &ValueFromSource{
									SecretKeyRef: &SecretKeyRef{
										Name: "creds", Key: "user",
									},
								},
							},
							Password: ValueType{
								ValueFrom: &ValueFromSource{
									SecretKeyRef: &SecretKeyRef{
										Namespace: "default", Key: "password",
									},
								},
							},
						},
					},
				},
			},
			expected: []SecretKeyRef{
				{Name: "creds", Namespace: "default"},
				{Name: "creds", Key: "user"},
				{Namespace: "default", Key: "password"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := test.given.GetSecretRefs()
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}

func TestTracePipeline_GetSecretRefs(t *testing.T) {
	tests := []struct {
		name         string
		given        *OTLPOutput
		pipelineName string
		expected     []SecretKeyRef
	}{
		{
			name:         "only endpoint",
			pipelineName: "test-pipeline",
			given: &OTLPOutput{
				Endpoint: ValueType{
					Value: "",
					ValueFrom: &ValueFromSource{
						SecretKeyRef: &SecretKeyRef{
							Name: "secret-1",
							Key:  "endpoint",
						}},
				},
			},

			expected: []SecretKeyRef{
				{Name: "secret-1", Key: "endpoint"},
			},
		},
		{
			name:         "basic auth and header",
			pipelineName: "test-pipeline",
			given: &OTLPOutput{
				Authentication: &AuthenticationOptions{
					Basic: &BasicAuthOptions{
						User: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name:      "secret-1",
									Namespace: "default",
									Key:       "user",
								}},
						},
						Password: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name:      "secret-2",
									Namespace: "default",
									Key:       "password",
								}},
						},
					},
				},
				Headers: []Header{
					{
						Name: "header-1",
						ValueType: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name:      "secret-3",
									Namespace: "default",
									Key:       "myheader",
								}},
						},
					},
				},
			},

			expected: []SecretKeyRef{
				{Name: "secret-1", Namespace: "default", Key: "user"},
				{Name: "secret-2", Namespace: "default", Key: "password"},
				{Name: "secret-3", Namespace: "default", Key: "myheader"},
			},
		},
		{
			name:         "basic auth and header (with missing fields)",
			pipelineName: "test-pipeline",
			given: &OTLPOutput{
				Authentication: &AuthenticationOptions{
					Basic: &BasicAuthOptions{
						User: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name: "secret-1",
									Key:  "user",
								}},
						},
						Password: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Namespace: "default",
									Key:       "password",
								}},
						},
					},
				},
				Headers: []Header{
					{
						Name: "header-1",
						ValueType: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name:      "secret-3",
									Namespace: "default",
								}},
						},
					},
				},
			},

			expected: []SecretKeyRef{
				{Name: "secret-1", Key: "user"},
				{Namespace: "default", Key: "password"},
				{Name: "secret-3", Namespace: "default"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sut := TracePipeline{ObjectMeta: metav1.ObjectMeta{Name: test.pipelineName}, Spec: TracePipelineSpec{Output: TracePipelineOutput{OTLP: test.given}}}
			actual := sut.GetSecretRefs()
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}

func TestMetricPipeline_GetSecretRefs(t *testing.T) {
	tests := []struct {
		name         string
		given        *OTLPOutput
		pipelineName string
		expected     []SecretKeyRef
	}{
		{
			name:         "only endpoint",
			pipelineName: "test-pipeline",
			given: &OTLPOutput{
				Endpoint: ValueType{
					Value: "",
					ValueFrom: &ValueFromSource{
						SecretKeyRef: &SecretKeyRef{
							Name: "secret-1",
							Key:  "endpoint",
						}},
				},
			},

			expected: []SecretKeyRef{
				{Name: "secret-1", Key: "endpoint"},
			},
		},
		{
			name:         "basic auth and header",
			pipelineName: "test-pipeline",
			given: &OTLPOutput{
				Authentication: &AuthenticationOptions{
					Basic: &BasicAuthOptions{
						User: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name:      "secret-1",
									Namespace: "default",
									Key:       "user",
								}},
						},
						Password: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name:      "secret-2",
									Namespace: "default",
									Key:       "password",
								}},
						},
					},
				},
				Headers: []Header{
					{
						Name: "header-1",
						ValueType: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name:      "secret-3",
									Namespace: "default",
									Key:       "myheader",
								}},
						},
					},
				},
			},

			expected: []SecretKeyRef{
				{Name: "secret-1", Namespace: "default", Key: "user"},
				{Name: "secret-2", Namespace: "default", Key: "password"},
				{Name: "secret-3", Namespace: "default", Key: "myheader"},
			},
		},
		{
			name:         "basic auth and header (with missing fields)",
			pipelineName: "test-pipeline",
			given: &OTLPOutput{
				Authentication: &AuthenticationOptions{
					Basic: &BasicAuthOptions{
						User: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Namespace: "default",
									Key:       "user",
								}},
						},
						Password: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name: "secret-2",
									Key:  "password",
								}},
						},
					},
				},
				Headers: []Header{
					{
						Name: "header-1",
						ValueType: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name:      "secret-3",
									Namespace: "default",
								}},
						},
					},
				},
			},

			expected: []SecretKeyRef{
				{Namespace: "default", Key: "user"},
				{Name: "secret-2", Key: "password"},
				{Name: "secret-3", Namespace: "default"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sut := MetricPipeline{ObjectMeta: metav1.ObjectMeta{Name: test.pipelineName}, Spec: MetricPipelineSpec{Output: MetricPipelineOutput{OTLP: test.given}}}
			actual := sut.GetSecretRefs()
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}
