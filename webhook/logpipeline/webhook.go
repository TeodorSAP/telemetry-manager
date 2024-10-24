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

package logpipeline

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/webhook/logpipeline/validation"
)

const (
	StatusReasonConfigurationError = "InvalidConfiguration"
)

// +kubebuilder:webhook:path=/validate-logpipeline,mutating=false,failurePolicy=fail,sideEffects=None,groups=telemetry.kyma-project.io,resources=logpipelines,verbs=create;update,versions=v1alpha1,name=vlogpipeline.kb.io,admissionReviewVersions=v1
type ValidatingWebhookHandler struct {
	client.Client
	variablesValidator    validation.VariablesValidator
	maxPipelinesValidator validation.MaxPipelinesValidator
	fileValidator         validation.FilesValidator
	decoder               admission.Decoder
}

func NewValidatingWebhookHandler(
	client client.Client,
	variablesValidator validation.VariablesValidator,
	maxPipelinesValidator validation.MaxPipelinesValidator,
	fileValidator validation.FilesValidator,
	decoder admission.Decoder,
) *ValidatingWebhookHandler {
	return &ValidatingWebhookHandler{
		Client:                client,
		variablesValidator:    variablesValidator,
		maxPipelinesValidator: maxPipelinesValidator,
		decoder:               decoder,
		fileValidator:         fileValidator,
	}
}

func (v *ValidatingWebhookHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := logf.FromContext(ctx)

	logPipeline := &telemetryv1alpha1.LogPipeline{}
	if err := v.decoder.Decode(req, logPipeline); err != nil {
		log.Error(err, "Failed to decode LogPipeline")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := v.validateLogPipeline(ctx, logPipeline); err != nil {
		log.Error(err, "LogPipeline rejected")

		return admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Code:    int32(http.StatusForbidden),
					Reason:  StatusReasonConfigurationError,
					Message: err.Error(),
				},
			},
		}
	}

	var warnMsg []string

	if logPipeline.ContainsCustomPlugin() {
		helpText := "https://kyma-project.io/#/telemetry-manager/user/02-logs"
		msg := fmt.Sprintf("Logpipeline '%s' uses unsupported custom filters or outputs. We recommend changing the pipeline to use supported filters or output. See the documentation: %s", logPipeline.Name, helpText)
		warnMsg = append(warnMsg, msg)
	}

	if len(warnMsg) != 0 {
		return admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed:  true,
				Warnings: warnMsg,
			},
		}
	}

	return admission.Allowed("LogPipeline validation successful")
}

func (v *ValidatingWebhookHandler) validateLogPipeline(ctx context.Context, logPipeline *telemetryv1alpha1.LogPipeline) error {
	log := logf.FromContext(ctx)

	var logPipelines telemetryv1alpha1.LogPipelineList
	if err := v.List(ctx, &logPipelines); err != nil {
		return err
	}

	if err := v.maxPipelinesValidator.Validate(logPipeline, &logPipelines); err != nil {
		log.Error(err, "Maximum number of log pipelines reached")
		return err
	}

	if err := logPipeline.Validate(); err != nil {
		log.Error(err, "Failed to validate Fluent Bit input")
		return err
	}

	if err := v.variablesValidator.Validate(logPipeline, &logPipelines); err != nil {
		log.Error(err, "Failed to validate variables")
		return err
	}

	if err := v.fileValidator.Validate(logPipeline, &logPipelines); err != nil {
		log.Error(err, "Failed to validate Fluent Bit config")
		return err
	}

	return nil
}
