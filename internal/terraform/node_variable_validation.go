// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package terraform

import (
	"fmt"
	"slices"

	"github.com/hashicorp/terraform/internal/addrs"
	"github.com/hashicorp/terraform/internal/configs"
	"github.com/hashicorp/terraform/internal/lang/langrefs"
)

// nodeVariableValidation checks the author-specified validation rules against
// the final value of all expanded instances of a given input variable.
//
// A node of this type should always depend on another node that's responsible
// for deciding the final values for the nominated variable and registering
// them in the current "named values" state. [variableValidationTransformer]
// is the one responsible for inserting nodes of this type and ensuring that
// they each depend on the node that will register the final variable value.
type nodeVariableValidation struct {
	configAddr addrs.ConfigInputVariable
	rules      []*configs.CheckRule
}

var _ GraphNodeModulePath = (*nodeVariableValidation)(nil)
var _ GraphNodeReferenceable = (*nodeVariableValidation)(nil)
var _ GraphNodeReferencer = (*nodeVariableValidation)(nil)

func (n *nodeVariableValidation) Name() string {
	return fmt.Sprintf("%s (validation)", n.configAddr.String())
}

// ModulePath implements [GraphNodeModulePath].
func (n *nodeVariableValidation) ModulePath() addrs.Module {
	return n.configAddr.Module
}

// ReferenceableAddrs implements [GraphNodeReferenceable], announcing that
// this node contributes to the value for the input variable that it's
// validating, and must therefore run before any nodes that refer to it.
func (n *nodeVariableValidation) ReferenceableAddrs() []addrs.Referenceable {
	return []addrs.Referenceable{n.configAddr.Variable}
}

// References implements [GraphNodeReferencer], announcing anything that
// the check rules refer to, other than the variable that's being validated
// (which gets its dependency connected by [variableValidationTransformer]
// instead).
func (n *nodeVariableValidation) References() []*addrs.Reference {
	var ret []*addrs.Reference
	for _, rule := range n.rules {
		// We ignore all diagnostics here because if an expression contains
		// invalid references then we'll catch them once we visit the
		// node (method Execute).
		condRefs, _ := langrefs.ReferencesInExpr(addrs.ParseRef, rule.Condition)
		msgRefs, _ := langrefs.ReferencesInExpr(addrs.ParseRef, rule.ErrorMessage)
		ret = n.appendRefsFilterSelf(ret, condRefs...)
		ret = n.appendRefsFilterSelf(ret, msgRefs...)
	}
	return ret
}

// appendRefsFilterSelf is a specialized version of builtin [append] that
// ignores any new references to the input variable represented by the
// reciever.
func (n *nodeVariableValidation) appendRefsFilterSelf(to []*addrs.Reference, new ...*addrs.Reference) []*addrs.Reference {
	// We need to filter out any self-references, because those would
	// make the resulting graph invalid and we don't need them because
	// variableValidationTransformer should've arranged for us to
	// already depend on whatever node provides the final value for
	// this variable.
	ret := slices.Grow(to, len(new))
	ourAddr := n.configAddr.Variable
	for _, ref := range new {
		if refAddr, ok := ref.Subject.(addrs.InputVariable); ok {
			if refAddr == ourAddr {
				continue
			}
		}
		ret = append(ret, ref)
	}
	return ret
}
