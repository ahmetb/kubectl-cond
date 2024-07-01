// Copyright 2024 Ahmet Alp Balkan
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

package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/utils/ptr"
)

var (
	bold = color.New(color.Bold)
)

func main() {
	configFlags := genericclioptions.NewConfigFlags(true)

	cmd := &cobra.Command{
		Use:   "kubectl cond",
		Short: "View Kubernetes resource conditions",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runFunc(configFlags),
	}
	configFlags.AddFlags(cmd.PersistentFlags())
	if err := cmd.Execute(); err != nil {
		fmt.Printf("command failed: %v\n", err)
		os.Exit(1)
	}
}

func runFunc(configFlags *genericclioptions.ConfigFlags) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, posArgs []string) error {
		namespace := ptr.Deref(configFlags.Namespace, "")
		return resource.NewBuilder(configFlags).
			Unstructured().
			NamespaceParam(namespace).DefaultNamespace().
			ResourceTypeOrNameArgs(true, posArgs...).
			Flatten().
			Do().
			Visit(func(info *resource.Info, err error) error {
				if err != nil {
					return fmt.Errorf("error visiting resource: %w", err)
				}
				return printObject(info.Object)
			})
	}
}

type GenericCondition struct {
	Type               string                 `json:"type"`
	Status             metav1.ConditionStatus `json:"status"`
	Reason             string                 `json:"reason"`
	Message            string                 `json:"message"`
	LastUpdateTime     *metav1.Time           `json:"lastUpdateTime"`
	LastTransitionTime *metav1.Time           `json:"lastTransitionTime"`
	ObservedGeneration int64                  `json:"observedGeneration"`
}

func printObject(obj runtime.Object) error {
	unstructuredObj, err := convertToUnstructured(obj)
	if err != nil {
		return err
	}

	conditions, err := extractConditions(unstructuredObj)
	if err != nil {
		return err
	}

	sort.Slice(conditions, func(i, j int) bool {
		return byCondition(conditions[i], conditions[j])
	})

	objMeta, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("failed to extract object metadata: %w", err)
	}
	fmt.Printf("%s/%s\n", obj.GetObjectKind().GroupVersionKind().Kind, objMeta.GetName())

	printConditions(conditions)
	return nil
}

func convertToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		objJSON, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to convert object to unstructured: %w", err)
		}
		unstructuredObj = &unstructured.Unstructured{Object: objJSON}
	}
	return unstructuredObj, nil
}

func extractConditions(unstructuredObj *unstructured.Unstructured) ([]GenericCondition, error) {
	conditions, found, err := unstructured.NestedSlice(unstructuredObj.Object, "status", "conditions")
	if err != nil {
		return nil, fmt.Errorf("failed to extract conditions from object: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("no status.conditions[] found in object")
	}

	var condElems []GenericCondition
	for i, c := range conditions {
		condMap, ok := c.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to convert condition#%d to map (type: %T)", i, c)
		}
		var cond GenericCondition
		if err := convertMapToStruct(condMap, &cond); err != nil {
			return nil, fmt.Errorf("failed to convert condition#%d: %w", i, err)
		}
		condElems = append(condElems, cond)
	}
	return condElems, nil
}

func convertMapToStruct(condMap map[string]interface{}, cond *GenericCondition) error {
	b, err := json.Marshal(condMap)
	if err != nil {
		return fmt.Errorf("failed to marshal condition: %w", err)
	}
	if err := json.Unmarshal(b, cond); err != nil {
		return fmt.Errorf("failed to unmarshal condition: %w", err)
	}
	return nil
}

func printConditions(conditions []GenericCondition) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Condition Type", "Details"})
	table.SetColWidth(100)
	table.SetAutoWrapText(false)
	table.SetRowLine(true)

	for _, cond := range conditions {
		condType := statusColor(cond.Status)(cond.Type) + "\n" + "(" + string(cond.Status) + ")"
		details := formatConditionDetails(cond)
		table.Append([]string{condType, details})
	}

	table.Render()
}

func statusColor(status metav1.ConditionStatus) func(string) string {
	var statusColor *color.Color
	switch status {
	case metav1.ConditionTrue:
		statusColor = color.New(color.FgGreen)
	case metav1.ConditionFalse:
		statusColor = color.New(color.FgRed)
	case metav1.ConditionUnknown:
		statusColor = color.New(color.FgHiBlack)
	default: // shouldn't happen in practice
		statusColor = color.New(color.FgHiBlack)
	}
	return func(s string) string {
		return statusColor.Sprint(s)
	}
}

func formatConditionDetails(cond GenericCondition) string {
	color := statusColor(cond.Status)
	var details []string
	if cond.Reason != "" {
		details = append(details, color(bold.Sprintf("Reason: %s", cond.Reason)))
	}
	if cond.Message != "" {
		wrappedMessage := wrapString(cond.Message, 80)
		details = append(details, color(fmt.Sprintf("Message: (%s)", wrappedMessage)))
	}
	if cond.LastTransitionTime != nil {
		details = append(details, fmt.Sprintf("* Last Transition: %s (%s ago)", cond.LastTransitionTime.Time.Format(time.RFC3339), time.Since(cond.LastTransitionTime.Time).Round(time.Second)))
	}
	if cond.LastUpdateTime != nil {
		details = append(details, fmt.Sprintf("* Last Update: %s (%s ago)", cond.LastUpdateTime.Time.Format(time.RFC3339), time.Since(cond.LastUpdateTime.Time).Round(time.Second)))
	}
	return strings.Join(details, "\n")
}

func byCondition(i, j GenericCondition) bool {
	// Rule 1: prioritize specific types
	typePriority := map[string]int{
		"Ready":     -2,
		"Succeeded": -1, // e.g. Job
	}
	priI, priJ := typePriority[i.Type], typePriority[j.Type]

	if priI != priJ {
		return priI < priJ
	}

	// Rule 2: status=False first, then Unknown, then True
	statusOrder := map[metav1.ConditionStatus]int{
		metav1.ConditionFalse:   0,
		metav1.ConditionUnknown: 1,
		metav1.ConditionTrue:    2,
	}
	if statusOrder[i.Status] != statusOrder[j.Status] {
		return statusOrder[i.Status] < statusOrder[j.Status]
	}

	// Rule 3: Sort by the last time it got changed in descending order
	timeI := ptr.Deref(i.LastUpdateTime, ptr.Deref(i.LastTransitionTime, metav1.Time{})).Time
	timeJ := ptr.Deref(j.LastUpdateTime, ptr.Deref(j.LastTransitionTime, metav1.Time{})).Time
	return timeI.After(timeJ)
}

func wrapString(input string, n int) string {
	if n <= 0 {
		return input
	}

	var result strings.Builder
	for i, char := range input {
		if i > 0 && i%n == 0 {
			result.WriteString("\n")
		}
		result.WriteRune(char)
	}

	return result.String()
}
