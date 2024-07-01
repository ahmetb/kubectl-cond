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
					return err
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
	// Convert the object to unstructured if it is not already
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		// Object is not unstructured, convert it
		objJSON, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return fmt.Errorf("failed to convert object to unstructured: %w", err)
		}
		unstructuredObj = &unstructured.Unstructured{Object: objJSON}
	}

	// Extract status.conditions from the unstructured object
	conditions, found, err := unstructured.NestedSlice(unstructuredObj.Object, "status", "conditions")
	if err != nil {
		return fmt.Errorf("failed to extract conditions from object: %w", err)
	}
	if !found {
		return fmt.Errorf("no status.conditions[] found in object")
	}

	condElems := make([]GenericCondition, 0, len(conditions))
	for i, c := range conditions {
		condMap, ok := c.(map[string]any)
		if !ok {
			return fmt.Errorf("failed to convert condition#%d to map (type: %T)", i, c)
		}
		// convert untyped map to GenericCondition
		b, err := json.Marshal(condMap)
		if err != nil {
			return fmt.Errorf("failed to marshal condition#%d: %w", i, err)
		}
		var c GenericCondition
		if err := json.Unmarshal(b, &c); err != nil {
			return fmt.Errorf("failed to unmarshal condition#%d: %w", i, err)
		}
		condElems = append(condElems, c)
	}

	sort.Slice(condElems, func(i, j int) bool {
		return byCondition(condElems[i], condElems[j])
	})

	objMeta, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("failed to extract object metadata: %w", err)
	}
	fmt.Printf("%s/%s\n", obj.GetObjectKind().GroupVersionKind().Kind, objMeta.GetName())

	printConditions(condElems)
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
	colorize := statusColor(cond.Status)
	var detail string
	if cond.Reason != "" {
		detail += fmt.Sprintf("%s\n", colorize(bold.Sprint(cond.Reason)))
	}
	if cond.Message != "" {
		cond.Message = wrapString(cond.Message, 80, colorize)
		cond.Message = colorize("(") + cond.Message + colorize(")")
		detail += fmt.Sprintf("%s\n", cond.Message)
	}
	if cond.LastTransitionTime != nil {
		detail += fmt.Sprintf("* Last Transition: %s (%s ago)\n", cond.LastTransitionTime.Time.Format(time.RFC3339), time.Since(cond.LastTransitionTime.Time).Round(time.Second))
	}
	if cond.LastUpdateTime != nil {
		detail += fmt.Sprintf("* Last Update: %s (%s ago)\n", cond.LastUpdateTime.Time.Format(time.RFC3339), time.Since(cond.LastUpdateTime.Time).Round(time.Second))
	}
	detail = strings.TrimSuffix(detail, "\n")
	return detail
}

func byCondition(i, j GenericCondition) bool {
	// Rule 1: prioritize specific types
	typePriority := map[string]int{
		"Ready":     -2,
		"Succeeded": -1, // e.g. Job
	}
	priI := typePriority[i.Type]
	priJ := typePriority[j.Type]

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

// wrapString wraps the input string to a given width n, splitting long words as needed.
func wrapString[T ~string](input T, n int, colorize func(string) string) T {
	if n <= 0 {
		return input
	}

	var result strings.Builder
	var line strings.Builder
	lineLength := 0

	for _, char := range input {
		line.WriteRune(char)
		lineLength++

		if lineLength >= n || char == '\n' {
			result.WriteString(colorize(line.String()))
			line.Reset()
			result.WriteString("\n")
			lineLength = 0
		}
	}

	if line.Len() > 0 {
		result.WriteString(colorize(line.String()))
	}

	return T(result.String())
}
