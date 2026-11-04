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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func field(id int64, name string) icebergTableSchemaField {
	f := icebergTableSchemaField{Name: name, Type: "long", Required: false}
	if id == 0 {
		f.ID = types.Int64Unknown()
	} else {
		f.ID = types.Int64Value(id)
	}

	return f
}

func structField(id int64, name string, fields ...icebergTableSchemaField) icebergTableSchemaField {
	f := field(id, name)
	f.Type = "struct"
	f.StructProperties = &icebergTableSchemaFieldStructProperties{Fields: fields}

	return f
}

func TestDroppedFields(t *testing.T) {
	state := icebergTableSchema{Fields: []icebergTableSchemaField{
		field(1, "id"),
		field(2, "count"),
		structField(3, "meta", field(4, "source"), field(5, "region")),
	}}

	for name, tc := range map[string]struct {
		plan []icebergTableSchemaField
		want []string
	}{
		"unchanged": {
			plan: []icebergTableSchemaField{field(1, "id"), field(2, "count"), structField(3, "meta", field(4, "source"), field(5, "region"))},
		},
		"ids omitted, matched by name": {
			plan: []icebergTableSchemaField{field(0, "id"), field(0, "count"), structField(0, "meta", field(0, "source"), field(0, "region"))},
		},
		"added column": {
			plan: []icebergTableSchemaField{field(1, "id"), field(2, "count"), structField(3, "meta", field(4, "source"), field(5, "region")), field(0, "hits")},
		},
		"rename keeping the id": {
			plan: []icebergTableSchemaField{field(1, "id"), field(2, "hits"), structField(3, "meta", field(4, "source"), field(5, "region"))},
		},
		"rename without the id": {
			plan: []icebergTableSchemaField{field(1, "id"), field(0, "hits"), structField(3, "meta", field(4, "source"), field(5, "region"))},
			want: []string{`"count" (id 2)`},
		},
		"removed column": {
			plan: []icebergTableSchemaField{field(1, "id"), structField(3, "meta", field(4, "source"), field(5, "region"))},
			want: []string{`"count" (id 2)`},
		},
		"removed nested field": {
			plan: []icebergTableSchemaField{field(1, "id"), field(2, "count"), structField(3, "meta", field(4, "source"))},
			want: []string{`"meta.region" (id 5)`},
		},
		"removed struct reports the struct only": {
			plan: []icebergTableSchemaField{field(1, "id"), field(2, "count")},
			want: []string{`"meta" (id 3)`},
		},
	} {
		t.Run(name, func(t *testing.T) {
			prior := state
			plan := icebergTableSchema{Fields: tc.plan}
			assert.Equal(t, tc.want, droppedFields(&plan, &prior))
		})
	}
}
