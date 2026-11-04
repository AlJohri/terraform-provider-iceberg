// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ resource.ResourceWithModifyPlan = &icebergTableResource{}

// ModifyPlan warns when an update drops a column. A field that is renamed
// without keeping its id is planned as a drop plus an add, which reads like a
// rename in the plan output.
func (r *icebergTableResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var planObj, stateObj types.Object
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("schema"), &planObj)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("schema"), &stateObj)...)
	if resp.Diagnostics.HasError() || planObj.IsUnknown() || planObj.IsNull() || stateObj.IsNull() {
		return
	}

	var planSchema, stateSchema icebergTableSchema
	resp.Diagnostics.Append(planObj.As(ctx, &planSchema, basetypes.ObjectAsOptions{})...)
	resp.Diagnostics.Append(stateObj.As(ctx, &stateSchema, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	dropped := droppedFields(&planSchema, &stateSchema)
	if len(dropped) == 0 {
		return
	}

	resp.Diagnostics.AddAttributeWarning(
		path.Root("schema"),
		"Columns will be dropped",
		fmt.Sprintf("This plan drops %s. Iceberg identifies a column by its field id, so the data of a dropped "+
			"column can no longer be read, also if a column with the same name is added later. To rename a column, "+
			"keep its id in the field.", strings.Join(dropped, ", ")),
	)
}

// droppedFields lists the state's fields whose ids the plan no longer uses,
// matching unset plan ids by name as Update does.
func droppedFields(plan, state *icebergTableSchema) []string {
	plan.resolveFieldIDs(state)

	kept := map[int64]bool{}
	var collect func(fields []icebergTableSchemaField)
	collect = func(fields []icebergTableSchemaField) {
		for _, f := range fields {
			if !fieldIDUnset(f.ID) {
				kept[f.ID.ValueInt64()] = true
			}
			if f.StructProperties != nil {
				collect(f.StructProperties.Fields)
			}
		}
	}
	collect(plan.Fields)

	var dropped []string
	var walk func(prefix string, fields []icebergTableSchemaField)
	walk = func(prefix string, fields []icebergTableSchemaField) {
		for _, f := range fields {
			name := prefix + f.Name
			if fieldIDUnset(f.ID) || kept[f.ID.ValueInt64()] {
				if f.StructProperties != nil {
					walk(name+".", f.StructProperties.Fields)
				}

				continue
			}
			dropped = append(dropped, fmt.Sprintf("%q (id %d)", name, f.ID.ValueInt64()))
		}
	}
	walk("", state.Fields)
	sort.Strings(dropped)

	return dropped
}
