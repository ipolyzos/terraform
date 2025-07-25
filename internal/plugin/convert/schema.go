// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package convert

import (
	"encoding/json"
	"reflect"
	"sort"

	"github.com/hashicorp/terraform/internal/configs/configschema"
	"github.com/hashicorp/terraform/internal/providers"
	proto "github.com/hashicorp/terraform/internal/tfplugin5"
)

// ConfigSchemaToProto takes a *configschema.Block and converts it to a
// proto.Schema_Block for a grpc response.
func ConfigSchemaToProto(b *configschema.Block) *proto.Schema_Block {
	block := &proto.Schema_Block{
		Description:     b.Description,
		DescriptionKind: protoStringKind(b.DescriptionKind),
		Deprecated:      b.Deprecated,
	}

	for _, name := range sortedKeys(b.Attributes) {
		a := b.Attributes[name]

		attr := &proto.Schema_Attribute{
			Name:            name,
			Description:     a.Description,
			DescriptionKind: protoStringKind(a.DescriptionKind),
			Optional:        a.Optional,
			Computed:        a.Computed,
			Required:        a.Required,
			Sensitive:       a.Sensitive,
			Deprecated:      a.Deprecated,
			WriteOnly:       a.WriteOnly,
		}

		ty, err := json.Marshal(a.Type)
		if err != nil {
			panic(err)
		}

		attr.Type = ty

		block.Attributes = append(block.Attributes, attr)
	}

	for _, name := range sortedKeys(b.BlockTypes) {
		b := b.BlockTypes[name]
		block.BlockTypes = append(block.BlockTypes, protoSchemaNestedBlock(name, b))
	}

	return block
}

func protoStringKind(k configschema.StringKind) proto.StringKind {
	switch k {
	default:
		return proto.StringKind_PLAIN
	case configschema.StringMarkdown:
		return proto.StringKind_MARKDOWN
	}
}

func protoSchemaNestedBlock(name string, b *configschema.NestedBlock) *proto.Schema_NestedBlock {
	var nesting proto.Schema_NestedBlock_NestingMode
	switch b.Nesting {
	case configschema.NestingSingle:
		nesting = proto.Schema_NestedBlock_SINGLE
	case configschema.NestingGroup:
		nesting = proto.Schema_NestedBlock_GROUP
	case configschema.NestingList:
		nesting = proto.Schema_NestedBlock_LIST
	case configschema.NestingSet:
		nesting = proto.Schema_NestedBlock_SET
	case configschema.NestingMap:
		nesting = proto.Schema_NestedBlock_MAP
	default:
		nesting = proto.Schema_NestedBlock_INVALID
	}
	return &proto.Schema_NestedBlock{
		TypeName: name,
		Block:    ConfigSchemaToProto(&b.Block),
		Nesting:  nesting,
		MinItems: int64(b.MinItems),
		MaxItems: int64(b.MaxItems),
	}
}

// ProtoToProviderSchema takes a proto.Schema and converts it to a providers.Schema.
// It takes an optional resource identity schema for resources that support identity.
func ProtoToProviderSchema(s *proto.Schema, id *proto.ResourceIdentitySchema) providers.Schema {
	schema := providers.Schema{
		Version: s.Version,
		Body:    ProtoToConfigSchema(s.Block),
	}

	if id != nil {
		schema.IdentityVersion = id.Version
		schema.Identity = ProtoToIdentitySchema(id.IdentityAttributes)
	}

	return schema
}

func ProtoToExecutionOrder(s proto.ActionSchema_Lifecycle_ExecutionOrder) providers.ExecutionOrder {
	switch s {
	case proto.ActionSchema_Lifecycle_INVALID:
		return providers.ExecutionOrderInvalid
	case proto.ActionSchema_Lifecycle_BEFORE:
		return providers.ExecutionOrderBefore
	case proto.ActionSchema_Lifecycle_AFTER:
		return providers.ExecutionOrderAfter
	default:
		panic("Unknown Execution Order, expected Invalid, Before, or After")
	}
}

func ExecutionOrderToProto(s providers.ExecutionOrder) proto.ActionSchema_Lifecycle_ExecutionOrder {
	switch s {
	case providers.ExecutionOrderInvalid:
		return proto.ActionSchema_Lifecycle_INVALID
	case providers.ExecutionOrderBefore:
		return proto.ActionSchema_Lifecycle_BEFORE
	case providers.ExecutionOrderAfter:
		return proto.ActionSchema_Lifecycle_AFTER
	default:
		panic("Unknown Execution Order, expected Invalid, Before, or After")
	}
}

func ProtoToLinkedResource(lr *proto.ActionSchema_LinkedResource) providers.LinkedResourceSchema {
	if lr == nil {
		return providers.LinkedResourceSchema{}
	}

	return providers.LinkedResourceSchema{
		TypeName: lr.TypeName,
	}
}

func LinkedResourceToProto(lr providers.LinkedResourceSchema) *proto.ActionSchema_LinkedResource {
	return &proto.ActionSchema_LinkedResource{
		TypeName: lr.TypeName,
	}
}

func ProtoToLinkedResources(lrs []*proto.ActionSchema_LinkedResource) []providers.LinkedResourceSchema {
	linkedResources := make([]providers.LinkedResourceSchema, len(lrs))
	for i, lr := range lrs {
		linkedResources[i] = ProtoToLinkedResource(lr)
	}
	return linkedResources
}

func LinkedResourcesToProto(lrs []providers.LinkedResourceSchema) []*proto.ActionSchema_LinkedResource {
	linkedResources := make([]*proto.ActionSchema_LinkedResource, len(lrs))
	for i, lr := range lrs {
		linkedResources[i] = LinkedResourceToProto(lr)
	}
	return linkedResources
}

func ProtoToActionSchema(s *proto.ActionSchema) providers.ActionSchema {
	schema := providers.ActionSchema{
		ConfigSchema: ProtoToConfigSchema(s.Schema.Block),
	}

	switch t := s.Type.(type) {
	case *proto.ActionSchema_Unlinked_:
		schema.Unlinked = &providers.UnlinkedAction{}
	case *proto.ActionSchema_Lifecycle_:
		schema.Lifecycle = &providers.LifecycleAction{
			Executes:       ProtoToExecutionOrder(t.Lifecycle.Executes),
			LinkedResource: ProtoToLinkedResource(t.Lifecycle.LinkedResource),
		}
	case *proto.ActionSchema_Linked_:
		schema.Linked = &providers.LinkedAction{
			LinkedResources: ProtoToLinkedResources(t.Linked.LinkedResources),
		}
	default:
		panic("Unknown Action Type, expected schema to contain either Unlinked, Liefecycle, or Linked")
	}
	return schema
}

// ProtoToConfigSchema takes the GetSchcema_Block from a grpc response and converts it
// to a terraform *configschema.Block.
func ProtoToConfigSchema(b *proto.Schema_Block) *configschema.Block {
	block := &configschema.Block{
		Attributes: make(map[string]*configschema.Attribute),
		BlockTypes: make(map[string]*configschema.NestedBlock),

		Description:     b.Description,
		DescriptionKind: schemaStringKind(b.DescriptionKind),
		Deprecated:      b.Deprecated,
	}

	for _, a := range b.Attributes {
		attr := &configschema.Attribute{
			Description:     a.Description,
			DescriptionKind: schemaStringKind(a.DescriptionKind),
			Required:        a.Required,
			Optional:        a.Optional,
			Computed:        a.Computed,
			Sensitive:       a.Sensitive,
			Deprecated:      a.Deprecated,
			WriteOnly:       a.WriteOnly,
		}

		if err := json.Unmarshal(a.Type, &attr.Type); err != nil {
			panic(err)
		}

		block.Attributes[a.Name] = attr
	}

	for _, b := range b.BlockTypes {
		block.BlockTypes[b.TypeName] = schemaNestedBlock(b)
	}

	return block
}

func schemaStringKind(k proto.StringKind) configschema.StringKind {
	switch k {
	default:
		return configschema.StringPlain
	case proto.StringKind_MARKDOWN:
		return configschema.StringMarkdown
	}
}

func schemaNestedBlock(b *proto.Schema_NestedBlock) *configschema.NestedBlock {
	var nesting configschema.NestingMode
	switch b.Nesting {
	case proto.Schema_NestedBlock_SINGLE:
		nesting = configschema.NestingSingle
	case proto.Schema_NestedBlock_GROUP:
		nesting = configschema.NestingGroup
	case proto.Schema_NestedBlock_LIST:
		nesting = configschema.NestingList
	case proto.Schema_NestedBlock_MAP:
		nesting = configschema.NestingMap
	case proto.Schema_NestedBlock_SET:
		nesting = configschema.NestingSet
	default:
		// In all other cases we'll leave it as the zero value (invalid) and
		// let the caller validate it and deal with this.
	}

	nb := &configschema.NestedBlock{
		Nesting:  nesting,
		MinItems: int(b.MinItems),
		MaxItems: int(b.MaxItems),
	}

	nested := ProtoToConfigSchema(b.Block)
	nb.Block = *nested
	return nb
}

// sortedKeys returns the lexically sorted keys from the given map. This is
// used to make schema conversions are deterministic. This panics if map keys
// are not a string.
func sortedKeys(m interface{}) []string {
	v := reflect.ValueOf(m)
	keys := make([]string, v.Len())

	mapKeys := v.MapKeys()
	for i, k := range mapKeys {
		keys[i] = k.Interface().(string)
	}

	sort.Strings(keys)
	return keys
}

func ProtoToIdentitySchema(attributes []*proto.ResourceIdentitySchema_IdentityAttribute) *configschema.Object {
	obj := &configschema.Object{
		Attributes: make(map[string]*configschema.Attribute),
		Nesting:    configschema.NestingSingle,
	}

	for _, a := range attributes {
		attr := &configschema.Attribute{
			Description: a.Description,
			Required:    a.RequiredForImport,
			Optional:    a.OptionalForImport,
		}

		if err := json.Unmarshal(a.Type, &attr.Type); err != nil {
			panic(err)
		}

		obj.Attributes[a.Name] = attr
	}

	return obj
}

func ResourceIdentitySchemaToProto(b providers.IdentitySchema) *proto.ResourceIdentitySchema {
	attrs := []*proto.ResourceIdentitySchema_IdentityAttribute{}
	for _, name := range sortedKeys(b.Body.Attributes) {
		a := b.Body.Attributes[name]

		attr := &proto.ResourceIdentitySchema_IdentityAttribute{
			Name:              name,
			Description:       a.Description,
			RequiredForImport: a.Required,
			OptionalForImport: a.Optional,
		}

		ty, err := json.Marshal(a.Type)
		if err != nil {
			panic(err)
		}

		attr.Type = ty

		attrs = append(attrs, attr)
	}

	return &proto.ResourceIdentitySchema{
		Version:            b.Version,
		IdentityAttributes: attrs,
	}
}
