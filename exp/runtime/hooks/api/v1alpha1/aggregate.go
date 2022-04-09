/*
Copyright 2022 The Kubernetes Authors.

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

package v1alpha1

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	runtimev1 "sigs.k8s.io/cluster-api/exp/runtime/api/v1alpha1"
)

type AggregatableResponse interface {
	runtime.Object

	Aggregate(items []*ExtensionResponse) (HookResponseSummary, error)
}

type ExtensionResponse struct {
	ExtensionName string
	Response      runtime.Object
	FailurePolicy *runtimev1.FailurePolicy
	Error         error
}

type ExtensionResponseList []*ExtensionResponse

type ResponseSummaryStatus string

const (
	ResponseSummaryStatusSuccess        ResponseSummaryStatus = "Success"
	ResponseSummaryStatusFailureIgnored ResponseSummaryStatus = "FailureIgnored"
	ResponseSummaryStatusFailure        ResponseSummaryStatus = "Failure"
)

type ExtensionResponseSummary struct {
	ExtensionName string
	Status        ResponseSummaryStatus
	Error         error
}

type HookResponseSummary []ExtensionResponseSummary

func aggregateExtensionResponses(response runtime.Object, responses []*ExtensionResponse) (HookResponseSummary, error) {
	msg := &strings.Builder{}
	status := ResponseStatusSuccess
	summaries := HookResponseSummary{}

	for _, item := range responses {
		ignore := *item.FailurePolicy == runtimev1.FailurePolicyIgnore
		summaryStatus := ResponseSummaryStatusSuccess

		if item.Error != nil {
			if !ignore {
				summaryStatus = ResponseSummaryStatusFailure
				status = ResponseStatusFailure
				m, err := GetMessage(item)
				if err != nil {
					return nil, err
				}
				msg.WriteString(m)
			} else {
				summaryStatus = ResponseSummaryStatusFailureIgnored
			}
		}

		summaries = append(summaries, ExtensionResponseSummary{
			ExtensionName: item.ExtensionName,
			Error:         item.Error,
			Status:        summaryStatus,
		})
	}

	if err := SetMessage(response, msg.String()); err != nil {
		return nil, err
	}
	if err := SetStatus(response, status); err != nil {
		return nil, err
	}
	return summaries, nil
}

func aggregateRetryAfterSeconds(response runtime.Object, responses []*ExtensionResponse) error {
	// TODO: Ignore responses that are failures.
	// TODO: For responses that are failures but are ignored should we even ignore their RetryAfterSeconds?
	for _, item := range responses {
		retry, err := GetRetryAfterSeconds(item)
		if err != nil {
			return err
		}
		current, err := GetRetryAfterSeconds(response)
		if err != nil {
			return err
		}
		if err := SetRetryAfterSeconds(
			response,
			lowestNonZeroRetryAfterSeconds(current, retry),
		); err != nil {
			return err
		}
	}
	return nil
}

func lowestNonZeroRetryAfterSeconds(i, j int) int {
	if i == 0 {
		return j
	}
	if j == 0 {
		return i
	}
	if i < j {
		return i
	}
	return j
}
