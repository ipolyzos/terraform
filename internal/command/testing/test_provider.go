// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testing

import (
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/zclconf/go-cty/cty"

	"github.com/hashicorp/terraform/internal/configs/configschema"
	"github.com/hashicorp/terraform/internal/providers"
	"github.com/hashicorp/terraform/internal/providers/testing"
	"github.com/hashicorp/terraform/internal/tfdiags"
)

var (
	ProviderSchema = &providers.GetProviderSchemaResponse{
		Provider: providers.Schema{
			Body: &configschema.Block{
				Attributes: map[string]*configschema.Attribute{
					"data_prefix":     {Type: cty.String, Optional: true},
					"resource_prefix": {Type: cty.String, Optional: true},
				},
			},
		},
		ResourceTypes: map[string]providers.Schema{
			"test_resource": {
				Body: &configschema.Block{
					Attributes: map[string]*configschema.Attribute{
						"id":                   {Type: cty.String, Optional: true, Computed: true},
						"value":                {Type: cty.String, Optional: true},
						"interrupt_count":      {Type: cty.Number, Optional: true},
						"destroy_fail":         {Type: cty.Bool, Optional: true, Computed: true},
						"create_wait_seconds":  {Type: cty.Number, Optional: true},
						"destroy_wait_seconds": {Type: cty.Number, Optional: true},
						"write_only":           {Type: cty.String, Optional: true, WriteOnly: true},
						"defer":                {Type: cty.Bool, Optional: true},
					},
				},
			},
		},
		DataSources: map[string]providers.Schema{
			"test_data_source": {
				Body: &configschema.Block{
					Attributes: map[string]*configschema.Attribute{
						"id":         {Type: cty.String, Required: true},
						"value":      {Type: cty.String, Computed: true},
						"write_only": {Type: cty.String, Optional: true, WriteOnly: true},

						// We never actually reference these values from a data
						// source, but we have tests that use the same cty.Value
						// to represent a test_resource and a test_data_source
						// so the schemas have to match.

						"interrupt_count":      {Type: cty.Number, Computed: true},
						"destroy_fail":         {Type: cty.Bool, Computed: true},
						"create_wait_seconds":  {Type: cty.Number, Computed: true},
						"destroy_wait_seconds": {Type: cty.Number, Computed: true},
						"defer":                {Type: cty.Bool, Computed: true},
					},
				},
			},
		},
		EphemeralResourceTypes: map[string]providers.Schema{
			"test_ephemeral_resource": {
				Body: &configschema.Block{
					Attributes: map[string]*configschema.Attribute{
						"value": {
							Type:     cty.String,
							Computed: true,
						},
					},
				},
			},
		},
		Functions: map[string]providers.FunctionDecl{
			"is_true": {
				Parameters: []providers.FunctionParam{
					{
						Name:               "input",
						Type:               cty.Bool,
						AllowNullValue:     false,
						AllowUnknownValues: false,
					},
				},
				ReturnType: cty.Bool,
			},
		},
	}
)

// TestProvider is a wrapper around terraform.MockProvider that defines dynamic
// schemas, and keeps track of the resources and data sources that it contains.
type TestProvider struct {
	Provider *testing.MockProvider

	data, resource cty.Value

	Interrupt chan<- struct{}

	Store *ResourceStore
}

// NewProvider creates a new TestProvider for use in tests.
//
// If you provide an empty or nil *ResourceStore argument this is equivalent to the provider
// not having provisioned any remote objects prior to the test's events.
//
// If you provide a *ResourceStore containing values, those cty.Values represent remote objects
// that the provider has 'already' provisioned and can return information about immediately in a test.
func NewProvider(store *ResourceStore) *TestProvider {
	if store == nil {
		store = &ResourceStore{
			Data: make(map[string]cty.Value),
		}
	}

	provider := &TestProvider{
		Provider: new(testing.MockProvider),
		Store:    store,
	}

	provider.Provider.GetProviderSchemaResponse = ProviderSchema
	provider.Provider.ConfigureProviderFn = provider.ConfigureProvider
	provider.Provider.PlanResourceChangeFn = provider.PlanResourceChange
	provider.Provider.ApplyResourceChangeFn = provider.ApplyResourceChange
	provider.Provider.ReadResourceFn = provider.ReadResource
	provider.Provider.ReadDataSourceFn = provider.ReadDataSource
	provider.Provider.CallFunctionFn = provider.CallFunction
	provider.Provider.OpenEphemeralResourceFn = provider.OpenEphemeralResource
	provider.Provider.CloseEphemeralResourceFn = provider.CloseEphemeralResource

	return provider
}

func (provider *TestProvider) DataPrefix() string {
	var prefix string
	if !provider.data.IsNull() && provider.data.IsKnown() {
		prefix = provider.data.AsString()
	}
	return prefix
}

func (provider *TestProvider) SetDataPrefix(prefix string) {
	provider.data = cty.StringVal(prefix)
}

func (provider *TestProvider) GetDataKey(id string) string {
	if !provider.data.IsNull() && provider.data.IsKnown() {
		return path.Join(provider.data.AsString(), id)
	}
	return id
}

func (provider *TestProvider) ResourcePrefix() string {
	var prefix string
	if !provider.resource.IsNull() && provider.resource.IsKnown() {
		prefix = provider.resource.AsString()
	}
	return prefix
}

func (provider *TestProvider) SetResourcePrefix(prefix string) {
	provider.resource = cty.StringVal(prefix)
}

func (provider *TestProvider) GetResourceKey(id string) string {
	if !provider.resource.IsNull() && provider.resource.IsKnown() {
		return path.Join(provider.resource.AsString(), id)
	}
	return id
}

func (provider *TestProvider) ResourceString() string {
	return provider.string(provider.ResourcePrefix())
}

func (provider *TestProvider) ResourceCount() int {
	return provider.count(provider.ResourcePrefix())
}

func (provider *TestProvider) DataSourceString() string {
	return provider.string(provider.DataPrefix())
}

func (provider *TestProvider) DataSourceCount() int {
	return provider.count(provider.DataPrefix())
}

func (provider *TestProvider) count(prefix string) int {
	defer provider.Store.beginRead()()

	if len(prefix) == 0 {
		return len(provider.Store.Data)
	}

	count := 0
	for key := range provider.Store.Data {
		if strings.HasPrefix(key, prefix) {
			count++
		}
	}
	return count
}

func (provider *TestProvider) string(prefix string) string {
	defer provider.Store.beginRead()()

	var keys []string
	for key := range provider.Store.Data {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	return strings.Join(keys, ", ")
}

func (provider *TestProvider) ConfigureProvider(request providers.ConfigureProviderRequest) providers.ConfigureProviderResponse {
	provider.resource = request.Config.GetAttr("resource_prefix")
	provider.data = request.Config.GetAttr("data_prefix")
	return providers.ConfigureProviderResponse{}
}

func (provider *TestProvider) PlanResourceChange(request providers.PlanResourceChangeRequest) providers.PlanResourceChangeResponse {
	if request.ProposedNewState.IsNull() {

		var deferred *providers.Deferred
		if shouldBeDeferred := request.PriorState.GetAttr("defer"); !shouldBeDeferred.IsNull() && shouldBeDeferred.True() {
			deferred = &providers.Deferred{
				Reason: providers.DeferredReasonResourceConfigUnknown,
			}
		}

		// Then this is a delete operation.
		return providers.PlanResourceChangeResponse{
			PlannedState: request.ProposedNewState,
			Deferred:     deferred,
		}
	}

	resource := request.ProposedNewState
	if id := resource.GetAttr("id"); !id.IsKnown() || id.IsNull() {
		vals := resource.AsValueMap()
		vals["id"] = cty.UnknownVal(cty.String)
		resource = cty.ObjectVal(vals)
	}

	if destroyFail := resource.GetAttr("destroy_fail"); !destroyFail.IsKnown() || destroyFail.IsNull() {
		vals := resource.AsValueMap()
		vals["destroy_fail"] = cty.UnknownVal(cty.Bool)
		resource = cty.ObjectVal(vals)
	}

	if writeOnly := resource.GetAttr("write_only"); !writeOnly.IsNull() {
		vals := resource.AsValueMap()
		vals["write_only"] = cty.NullVal(cty.String)
		resource = cty.ObjectVal(vals)
	}

	var deferred *providers.Deferred
	if shouldBeDeferred := resource.GetAttr("defer"); !shouldBeDeferred.IsKnown() || (!shouldBeDeferred.IsNull() && shouldBeDeferred.True()) {
		deferred = &providers.Deferred{
			Reason: providers.DeferredReasonResourceConfigUnknown,
		}
	}

	return providers.PlanResourceChangeResponse{
		PlannedState: resource,
		Deferred:     deferred,
	}
}

func (provider *TestProvider) ApplyResourceChange(request providers.ApplyResourceChangeRequest) providers.ApplyResourceChangeResponse {
	if request.PlannedState.IsNull() {
		// Then this is a delete operation.

		if destroyFail := request.PriorState.GetAttr("destroy_fail"); destroyFail.IsKnown() && destroyFail.True() {
			var diags tfdiags.Diagnostics
			diags = diags.Append(tfdiags.Sourceless(tfdiags.Error, "Failed to destroy resource", "destroy_fail is set to true"))
			return providers.ApplyResourceChangeResponse{
				Diagnostics: diags,
			}
		}

		if wait := request.PriorState.GetAttr("destroy_wait_seconds"); !wait.IsNull() && wait.IsKnown() {
			waitTime, _ := wait.AsBigFloat().Int64()
			time.Sleep(time.Second * time.Duration(waitTime))
		}

		provider.Store.Delete(provider.GetResourceKey(request.PriorState.GetAttr("id").AsString()))
		return providers.ApplyResourceChangeResponse{
			NewState: request.PlannedState,
		}
	}

	resource := request.PlannedState
	id := resource.GetAttr("id")
	if !id.IsKnown() {
		val, err := uuid.GenerateUUID()
		if err != nil {
			panic(fmt.Errorf("failed to generate uuid: %v", err))
		}

		id = cty.StringVal(val)

		vals := resource.AsValueMap()
		vals["id"] = id
		resource = cty.ObjectVal(vals)
	}

	if interrupts := resource.GetAttr("interrupt_count"); !interrupts.IsNull() && interrupts.IsKnown() && provider.Interrupt != nil {
		count, _ := interrupts.AsBigFloat().Int64()
		for ix := 0; ix < int(count); ix++ {
			provider.Interrupt <- struct{}{}
		}

		// Wait for a second to make sure the interrupts are processed by
		// Terraform before the provider finishes. This is an attempt to ensure
		// the output of any tests that rely on this behaviour is deterministic.
		time.Sleep(time.Second)
	}

	if wait := resource.GetAttr("create_wait_seconds"); !wait.IsNull() && wait.IsKnown() {
		waitTime, _ := wait.AsBigFloat().Int64()
		time.Sleep(time.Second * time.Duration(waitTime))
	}

	if destroyFail := resource.GetAttr("destroy_fail"); !destroyFail.IsKnown() {
		vals := resource.AsValueMap()
		vals["destroy_fail"] = cty.False
		resource = cty.ObjectVal(vals)
	}

	provider.Store.Put(provider.GetResourceKey(id.AsString()), resource)
	return providers.ApplyResourceChangeResponse{
		NewState: resource,
	}
}

func (provider *TestProvider) ReadResource(request providers.ReadResourceRequest) providers.ReadResourceResponse {
	var diags tfdiags.Diagnostics

	id := request.PriorState.GetAttr("id").AsString()
	resource := provider.Store.Get(provider.GetResourceKey(id))
	if resource == cty.NilVal {
		diags = diags.Append(tfdiags.Sourceless(tfdiags.Error, "not found", fmt.Sprintf("%s does not exist", id)))
	}

	return providers.ReadResourceResponse{
		NewState:    resource,
		Diagnostics: diags,
	}
}

func (provider *TestProvider) ReadDataSource(request providers.ReadDataSourceRequest) providers.ReadDataSourceResponse {
	var diags tfdiags.Diagnostics

	id := request.Config.GetAttr("id").AsString()
	resource := provider.Store.Get(provider.GetDataKey(id))
	if resource == cty.NilVal {
		diags = diags.Append(tfdiags.Sourceless(tfdiags.Error, "not found", fmt.Sprintf("%s does not exist", id)))
	}

	if writeOnly := resource.GetAttr("write_only"); !writeOnly.IsNull() {
		vals := resource.AsValueMap()
		vals["write_only"] = cty.NullVal(cty.String)
		resource = cty.ObjectVal(vals)
	}

	return providers.ReadDataSourceResponse{
		State:       resource,
		Diagnostics: diags,
	}
}

func (provider *TestProvider) CallFunction(request providers.CallFunctionRequest) providers.CallFunctionResponse {
	switch request.FunctionName {
	case "is_true":
		return providers.CallFunctionResponse{
			Result: request.Arguments[0],
		}
	default:
		return providers.CallFunctionResponse{
			Err: fmt.Errorf("unknown function %q", request.FunctionName),
		}
	}
}

func (provider *TestProvider) OpenEphemeralResource(providers.OpenEphemeralResourceRequest) (resp providers.OpenEphemeralResourceResponse) {
	resp.Result = cty.ObjectVal(map[string]cty.Value{
		"value": cty.StringVal("bar"),
	})
	return resp
}

func (provider *TestProvider) CloseEphemeralResource(providers.CloseEphemeralResourceRequest) (resp providers.CloseEphemeralResourceResponse) {
	return resp
}

// ResourceStore manages a set of cty.Value resources that can be shared between
// TestProvider providers.
//
// A ResourceStore represents the remote objects that a test provider is managing.
// For example, when the test provider gets a ReadResource request it will search
// the store for a resource with a matching ID. See (*TestProvider).ReadResource.
type ResourceStore struct {
	mutex sync.RWMutex

	Data map[string]cty.Value
}

func (store *ResourceStore) Delete(key string) cty.Value {
	defer store.beginWrite()()

	if resource, ok := store.Data[key]; ok {
		delete(store.Data, key)
		return resource
	}
	return cty.NilVal
}

func (store *ResourceStore) Get(key string) cty.Value {
	defer store.beginRead()()

	return store.get(key)
}

func (store *ResourceStore) Put(key string, resource cty.Value) cty.Value {
	defer store.beginWrite()()

	old := store.get(key)
	store.Data[key] = resource
	return old
}

func (store *ResourceStore) get(key string) cty.Value {
	if resource, ok := store.Data[key]; ok {
		return resource
	}
	return cty.NilVal
}

func (store *ResourceStore) beginWrite() func() {
	store.mutex.Lock()
	return store.mutex.Unlock

}
func (store *ResourceStore) beginRead() func() {
	store.mutex.RLock()
	return store.mutex.RUnlock
}
