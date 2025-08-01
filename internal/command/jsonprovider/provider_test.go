// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package jsonprovider

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/zclconf/go-cty/cty"

	"github.com/hashicorp/terraform/internal/configs/configschema"
	"github.com/hashicorp/terraform/internal/providers"
)

var cmpOpts = cmpopts.IgnoreUnexported(Provider{})

func TestMarshalProvider(t *testing.T) {
	tests := []struct {
		Input               providers.ProviderSchema
		IncludeExperimental bool
		Want                *Provider
	}{
		{
			providers.ProviderSchema{},
			false,
			&Provider{
				Provider:                 &Schema{},
				ResourceSchemas:          map[string]*Schema{},
				DataSourceSchemas:        map[string]*Schema{},
				EphemeralResourceSchemas: map[string]*Schema{},
				ResourceIdentitySchemas:  map[string]*IdentitySchema{},
				ActionSchemas:            map[string]*ActionSchema{},
			},
		},
		{
			testProvider(),
			false,
			&Provider{
				Provider: &Schema{
					Block: &Block{
						Attributes: map[string]*Attribute{
							"region": {
								AttributeType:   json.RawMessage(`"string"`),
								Required:        true,
								DescriptionKind: "plain",
							},
						},
						DescriptionKind: "plain",
					},
				},
				ResourceSchemas: map[string]*Schema{
					"test_instance": {
						Version: 42,
						Block: &Block{
							Attributes: map[string]*Attribute{
								"id": {
									AttributeType:   json.RawMessage(`"string"`),
									Optional:        true,
									Computed:        true,
									DescriptionKind: "plain",
								},
								"ami": {
									AttributeType:   json.RawMessage(`"string"`),
									Optional:        true,
									DescriptionKind: "plain",
								},
								"volumes": {
									AttributeNestedType: &NestedType{
										NestingMode: "list",
										Attributes: map[string]*Attribute{
											"size": {
												AttributeType:   json.RawMessage(`"string"`),
												Required:        true,
												DescriptionKind: "plain",
											},
											"mount_point": {
												AttributeType:   json.RawMessage(`"string"`),
												Required:        true,
												DescriptionKind: "plain",
											},
										},
									},
									Optional:        true,
									DescriptionKind: "plain",
								},
							},
							BlockTypes: map[string]*BlockType{
								"network_interface": {
									Block: &Block{
										Attributes: map[string]*Attribute{
											"device_index": {
												AttributeType:   json.RawMessage(`"string"`),
												Optional:        true,
												DescriptionKind: "plain",
											},
											"description": {
												AttributeType:   json.RawMessage(`"string"`),
												Optional:        true,
												DescriptionKind: "plain",
											},
										},
										DescriptionKind: "plain",
									},
									NestingMode: "list",
								},
							},
							DescriptionKind: "plain",
						},
					},
				},
				DataSourceSchemas: map[string]*Schema{
					"test_data_source": {
						Version: 3,
						Block: &Block{
							Attributes: map[string]*Attribute{
								"id": {
									AttributeType:   json.RawMessage(`"string"`),
									Optional:        true,
									Computed:        true,
									DescriptionKind: "plain",
								},
								"ami": {
									AttributeType:   json.RawMessage(`"string"`),
									Optional:        true,
									DescriptionKind: "plain",
								},
							},
							BlockTypes: map[string]*BlockType{
								"network_interface": {
									Block: &Block{
										Attributes: map[string]*Attribute{
											"device_index": {
												AttributeType:   json.RawMessage(`"string"`),
												Optional:        true,
												DescriptionKind: "plain",
											},
											"description": {
												AttributeType:   json.RawMessage(`"string"`),
												Optional:        true,
												DescriptionKind: "plain",
											},
										},
										DescriptionKind: "plain",
									},
									NestingMode: "list",
								},
							},
							DescriptionKind: "plain",
						},
					},
				},
				EphemeralResourceSchemas: map[string]*Schema{
					"test_eph_instance": {
						Block: &Block{
							Attributes: map[string]*Attribute{
								"id": {
									AttributeType:   json.RawMessage(`"string"`),
									Optional:        true,
									Computed:        true,
									DescriptionKind: "plain",
								},
								"ami": {
									AttributeType:   json.RawMessage(`"string"`),
									Optional:        true,
									DescriptionKind: "plain",
								},
								"volumes": {
									AttributeNestedType: &NestedType{
										NestingMode: "list",
										Attributes: map[string]*Attribute{
											"size": {
												AttributeType:   json.RawMessage(`"string"`),
												Required:        true,
												DescriptionKind: "plain",
											},
											"mount_point": {
												AttributeType:   json.RawMessage(`"string"`),
												Required:        true,
												DescriptionKind: "plain",
											},
										},
									},
									Optional:        true,
									DescriptionKind: "plain",
								},
							},
							BlockTypes: map[string]*BlockType{
								"network_interface": {
									Block: &Block{
										Attributes: map[string]*Attribute{
											"device_index": {
												AttributeType:   json.RawMessage(`"string"`),
												Optional:        true,
												DescriptionKind: "plain",
											},
											"description": {
												AttributeType:   json.RawMessage(`"string"`),
												Optional:        true,
												DescriptionKind: "plain",
											},
										},
										DescriptionKind: "plain",
									},
									NestingMode: "list",
								},
							},
							DescriptionKind: "plain",
						},
					},
				},
				ResourceIdentitySchemas: map[string]*IdentitySchema{},
				ActionSchemas:           map[string]*ActionSchema{},
			},
		},
		{
			providers.ProviderSchema{
				ListResourceTypes: map[string]providers.Schema{
					"test_list_resource": {
						Version: 1,
						Body: &configschema.Block{
							Attributes: map[string]*configschema.Attribute{
								"filter": {Type: cty.String, Optional: true},
								"items":  {Type: cty.List(cty.String), Required: true},
							},
						},
					},
				},
				Actions: map[string]providers.ActionSchema{
					"test_unlinked_action": {
						ConfigSchema: &configschema.Block{
							Attributes: map[string]*configschema.Attribute{
								"opt_attr": {Type: cty.String, Optional: true},
								"req_attr": {Type: cty.List(cty.String), Required: true},
							},
						},
						Unlinked: &providers.UnlinkedAction{},
					},
					"test_linked_action": {
						ConfigSchema: &configschema.Block{
							Attributes: map[string]*configschema.Attribute{
								"opt_attr": {Type: cty.String, Optional: true},
								"req_attr": {Type: cty.List(cty.String), Required: true},
							},
						},
						Linked: &providers.LinkedAction{
							LinkedResources: []providers.LinkedResourceSchema{
								{
									TypeName: "test_resource_a",
								},
								{
									TypeName: "test_resource_b",
								},
							},
						},
					},
					"test_lifecycle_action": {
						ConfigSchema: &configschema.Block{
							Attributes: map[string]*configschema.Attribute{
								"opt_attr": {Type: cty.String, Optional: true},
								"req_attr": {Type: cty.List(cty.String), Required: true},
							},
						},
						Lifecycle: &providers.LifecycleAction{
							LinkedResource: providers.LinkedResourceSchema{
								TypeName: "test_resource_c",
							},
							Executes: providers.ExecutionOrderBefore,
						},
					},
				},
			},
			true,
			&Provider{
				Provider:                 &Schema{},
				ResourceSchemas:          map[string]*Schema{},
				DataSourceSchemas:        map[string]*Schema{},
				EphemeralResourceSchemas: map[string]*Schema{},
				ListResourceSchemas: map[string]*Schema{
					"test_list_resource": {
						Version: 1,
						Block: &Block{
							Attributes: map[string]*Attribute{
								"filter": {
									AttributeType:   json.RawMessage(`"string"`),
									Optional:        true,
									DescriptionKind: "plain",
								},
								"items": {
									AttributeType:   json.RawMessage(`["list","string"]`),
									Required:        true,
									DescriptionKind: "plain",
								},
							},
							DescriptionKind: "plain",
						},
					},
				},
				ResourceIdentitySchemas: map[string]*IdentitySchema{},
				ActionSchemas: map[string]*ActionSchema{
					"test_unlinked_action": {
						ConfigSchema: &Block{
							Attributes: map[string]*Attribute{
								"opt_attr": {
									AttributeType:   json.RawMessage(`"string"`),
									Optional:        true,
									DescriptionKind: "plain",
								},
								"req_attr": {
									AttributeType:   json.RawMessage(`["list","string"]`),
									Required:        true,
									DescriptionKind: "plain",
								},
							},
							DescriptionKind: "plain",
						},
						Unlinked: &UnlinkedAction{},
					},
					"test_linked_action": {
						ConfigSchema: &Block{
							Attributes: map[string]*Attribute{
								"opt_attr": {
									AttributeType:   json.RawMessage(`"string"`),
									Optional:        true,
									DescriptionKind: "plain",
								},
								"req_attr": {
									AttributeType:   json.RawMessage(`["list","string"]`),
									Required:        true,
									DescriptionKind: "plain",
								},
							},
							DescriptionKind: "plain",
						},
						Linked: &LinkedAction{
							LinkedResources: []LinkedResourceSchema{
								{
									TypeName: "test_resource_a",
								},
								{
									TypeName: "test_resource_b",
								},
							},
						},
					},
					"test_lifecycle_action": {
						ConfigSchema: &Block{
							Attributes: map[string]*Attribute{
								"opt_attr": {
									AttributeType:   json.RawMessage(`"string"`),
									Optional:        true,
									DescriptionKind: "plain",
								},
								"req_attr": {
									AttributeType:   json.RawMessage(`["list","string"]`),
									Required:        true,
									DescriptionKind: "plain",
								},
							},
							DescriptionKind: "plain",
						},
						Lifecycle: &LifecycleAction{
							LinkedResource: LinkedResourceSchema{
								TypeName: "test_resource_c",
							},
							ExecutionOrder: "before",
						},
					},
				},
			},
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			got := marshalProvider(test.Input, test.IncludeExperimental)
			if diff := cmp.Diff(test.Want, got, cmpOpts); diff != "" {
				t.Fatalf("wrong result:\n %s\n", diff)
			}
		})
	}
}

func testProvider() providers.ProviderSchema {
	return providers.ProviderSchema{
		Provider: providers.Schema{
			Body: &configschema.Block{
				Attributes: map[string]*configschema.Attribute{
					"region": {Type: cty.String, Required: true},
				},
			},
		},
		ResourceTypes: map[string]providers.Schema{
			"test_instance": {
				Version: 42,
				Body: &configschema.Block{
					Attributes: map[string]*configschema.Attribute{
						"id":  {Type: cty.String, Optional: true, Computed: true},
						"ami": {Type: cty.String, Optional: true},
						"volumes": {
							Optional: true,
							NestedType: &configschema.Object{
								Nesting: configschema.NestingList,
								Attributes: map[string]*configschema.Attribute{
									"size":        {Type: cty.String, Required: true},
									"mount_point": {Type: cty.String, Required: true},
								},
							},
						},
					},
					BlockTypes: map[string]*configschema.NestedBlock{
						"network_interface": {
							Nesting: configschema.NestingList,
							Block: configschema.Block{
								Attributes: map[string]*configschema.Attribute{
									"device_index": {Type: cty.String, Optional: true},
									"description":  {Type: cty.String, Optional: true},
								},
							},
						},
					},
				},
			},
		},
		DataSources: map[string]providers.Schema{
			"test_data_source": {
				Version: 3,
				Body: &configschema.Block{
					Attributes: map[string]*configschema.Attribute{
						"id":  {Type: cty.String, Optional: true, Computed: true},
						"ami": {Type: cty.String, Optional: true},
					},
					BlockTypes: map[string]*configschema.NestedBlock{
						"network_interface": {
							Nesting: configschema.NestingList,
							Block: configschema.Block{
								Attributes: map[string]*configschema.Attribute{
									"device_index": {Type: cty.String, Optional: true},
									"description":  {Type: cty.String, Optional: true},
								},
							},
						},
					},
				},
			},
		},
		EphemeralResourceTypes: map[string]providers.Schema{
			"test_eph_instance": {
				Body: &configschema.Block{
					Attributes: map[string]*configschema.Attribute{
						"id":  {Type: cty.String, Optional: true, Computed: true},
						"ami": {Type: cty.String, Optional: true},
						"volumes": {
							Optional: true,
							NestedType: &configschema.Object{
								Nesting: configschema.NestingList,
								Attributes: map[string]*configschema.Attribute{
									"size":        {Type: cty.String, Required: true},
									"mount_point": {Type: cty.String, Required: true},
								},
							},
						},
					},
					BlockTypes: map[string]*configschema.NestedBlock{
						"network_interface": {
							Nesting: configschema.NestingList,
							Block: configschema.Block{
								Attributes: map[string]*configschema.Attribute{
									"device_index": {Type: cty.String, Optional: true},
									"description":  {Type: cty.String, Optional: true},
								},
							},
						},
					},
				},
			},
		},
		ListResourceTypes: map[string]providers.Schema{
			"test_list_resource": {
				Version: 1,
				Body: &configschema.Block{
					Attributes: map[string]*configschema.Attribute{
						"filter": {Type: cty.String, Optional: true},
						"items":  {Type: cty.List(cty.String), Required: true},
					},
				},
			},
		},
	}
}
