//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1beta1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Permission) DeepCopyInto(out *Permission) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Permission.
func (in *Permission) DeepCopy() *Permission {
	if in == nil {
		return nil
	}
	out := new(Permission)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Permissions) DeepCopyInto(out *Permissions) {
	*out = *in
	if in.Required != nil {
		in, out := &in.Required, &out.Required
		*out = make([]Permission, len(*in))
		copy(*out, *in)
	}
	if in.AdditionalScopes != nil {
		in, out := &in.AdditionalScopes, &out.AdditionalScopes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Permissions.
func (in *Permissions) DeepCopy() *Permissions {
	if in == nil {
		return nil
	}
	out := new(Permissions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessCheck) DeepCopyInto(out *SPIAccessCheck) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessCheck.
func (in *SPIAccessCheck) DeepCopy() *SPIAccessCheck {
	if in == nil {
		return nil
	}
	out := new(SPIAccessCheck)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SPIAccessCheck) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessCheckList) DeepCopyInto(out *SPIAccessCheckList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SPIAccessCheck, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessCheckList.
func (in *SPIAccessCheckList) DeepCopy() *SPIAccessCheckList {
	if in == nil {
		return nil
	}
	out := new(SPIAccessCheckList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SPIAccessCheckList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessCheckSpec) DeepCopyInto(out *SPIAccessCheckSpec) {
	*out = *in
	in.Permissions.DeepCopyInto(&out.Permissions)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessCheckSpec.
func (in *SPIAccessCheckSpec) DeepCopy() *SPIAccessCheckSpec {
	if in == nil {
		return nil
	}
	out := new(SPIAccessCheckSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessCheckStatus) DeepCopyInto(out *SPIAccessCheckStatus) {
	*out = *in
	if in.Tokens != nil {
		in, out := &in.Tokens, &out.Tokens
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessCheckStatus.
func (in *SPIAccessCheckStatus) DeepCopy() *SPIAccessCheckStatus {
	if in == nil {
		return nil
	}
	out := new(SPIAccessCheckStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessToken) DeepCopyInto(out *SPIAccessToken) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessToken.
func (in *SPIAccessToken) DeepCopy() *SPIAccessToken {
	if in == nil {
		return nil
	}
	out := new(SPIAccessToken)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SPIAccessToken) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessTokenBinding) DeepCopyInto(out *SPIAccessTokenBinding) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessTokenBinding.
func (in *SPIAccessTokenBinding) DeepCopy() *SPIAccessTokenBinding {
	if in == nil {
		return nil
	}
	out := new(SPIAccessTokenBinding)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SPIAccessTokenBinding) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessTokenBindingList) DeepCopyInto(out *SPIAccessTokenBindingList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SPIAccessTokenBinding, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessTokenBindingList.
func (in *SPIAccessTokenBindingList) DeepCopy() *SPIAccessTokenBindingList {
	if in == nil {
		return nil
	}
	out := new(SPIAccessTokenBindingList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SPIAccessTokenBindingList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessTokenBindingSpec) DeepCopyInto(out *SPIAccessTokenBindingSpec) {
	*out = *in
	in.Permissions.DeepCopyInto(&out.Permissions)
	in.Secret.DeepCopyInto(&out.Secret)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessTokenBindingSpec.
func (in *SPIAccessTokenBindingSpec) DeepCopy() *SPIAccessTokenBindingSpec {
	if in == nil {
		return nil
	}
	out := new(SPIAccessTokenBindingSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessTokenBindingStatus) DeepCopyInto(out *SPIAccessTokenBindingStatus) {
	*out = *in
	out.SyncedObjectRef = in.SyncedObjectRef
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessTokenBindingStatus.
func (in *SPIAccessTokenBindingStatus) DeepCopy() *SPIAccessTokenBindingStatus {
	if in == nil {
		return nil
	}
	out := new(SPIAccessTokenBindingStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessTokenDataUpdate) DeepCopyInto(out *SPIAccessTokenDataUpdate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessTokenDataUpdate.
func (in *SPIAccessTokenDataUpdate) DeepCopy() *SPIAccessTokenDataUpdate {
	if in == nil {
		return nil
	}
	out := new(SPIAccessTokenDataUpdate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SPIAccessTokenDataUpdate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessTokenDataUpdateList) DeepCopyInto(out *SPIAccessTokenDataUpdateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SPIAccessTokenDataUpdate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessTokenDataUpdateList.
func (in *SPIAccessTokenDataUpdateList) DeepCopy() *SPIAccessTokenDataUpdateList {
	if in == nil {
		return nil
	}
	out := new(SPIAccessTokenDataUpdateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SPIAccessTokenDataUpdateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessTokenDataUpdateSpec) DeepCopyInto(out *SPIAccessTokenDataUpdateSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessTokenDataUpdateSpec.
func (in *SPIAccessTokenDataUpdateSpec) DeepCopy() *SPIAccessTokenDataUpdateSpec {
	if in == nil {
		return nil
	}
	out := new(SPIAccessTokenDataUpdateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessTokenList) DeepCopyInto(out *SPIAccessTokenList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SPIAccessToken, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessTokenList.
func (in *SPIAccessTokenList) DeepCopy() *SPIAccessTokenList {
	if in == nil {
		return nil
	}
	out := new(SPIAccessTokenList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SPIAccessTokenList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessTokenSpec) DeepCopyInto(out *SPIAccessTokenSpec) {
	*out = *in
	in.Permissions.DeepCopyInto(&out.Permissions)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessTokenSpec.
func (in *SPIAccessTokenSpec) DeepCopy() *SPIAccessTokenSpec {
	if in == nil {
		return nil
	}
	out := new(SPIAccessTokenSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SPIAccessTokenStatus) DeepCopyInto(out *SPIAccessTokenStatus) {
	*out = *in
	if in.TokenMetadata != nil {
		in, out := &in.TokenMetadata, &out.TokenMetadata
		*out = new(TokenMetadata)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SPIAccessTokenStatus.
func (in *SPIAccessTokenStatus) DeepCopy() *SPIAccessTokenStatus {
	if in == nil {
		return nil
	}
	out := new(SPIAccessTokenStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretSpec) DeepCopyInto(out *SecretSpec) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	out.Fields = in.Fields
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretSpec.
func (in *SecretSpec) DeepCopy() *SecretSpec {
	if in == nil {
		return nil
	}
	out := new(SecretSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TargetObjectRef) DeepCopyInto(out *TargetObjectRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TargetObjectRef.
func (in *TargetObjectRef) DeepCopy() *TargetObjectRef {
	if in == nil {
		return nil
	}
	out := new(TargetObjectRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Token) DeepCopyInto(out *Token) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Token.
func (in *Token) DeepCopy() *Token {
	if in == nil {
		return nil
	}
	out := new(Token)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TokenFieldMapping) DeepCopyInto(out *TokenFieldMapping) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TokenFieldMapping.
func (in *TokenFieldMapping) DeepCopy() *TokenFieldMapping {
	if in == nil {
		return nil
	}
	out := new(TokenFieldMapping)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TokenMetadata) DeepCopyInto(out *TokenMetadata) {
	*out = *in
	if in.Scopes != nil {
		in, out := &in.Scopes, &out.Scopes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ServiceProviderState != nil {
		in, out := &in.ServiceProviderState, &out.ServiceProviderState
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TokenMetadata.
func (in *TokenMetadata) DeepCopy() *TokenMetadata {
	if in == nil {
		return nil
	}
	out := new(TokenMetadata)
	in.DeepCopyInto(out)
	return out
}
