//go:build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MaskinportenClient) DeepCopyInto(out *MaskinportenClient) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MaskinportenClient.
func (in *MaskinportenClient) DeepCopy() *MaskinportenClient {
	if in == nil {
		return nil
	}
	out := new(MaskinportenClient)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MaskinportenClient) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MaskinportenClientList) DeepCopyInto(out *MaskinportenClientList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MaskinportenClient, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MaskinportenClientList.
func (in *MaskinportenClientList) DeepCopy() *MaskinportenClientList {
	if in == nil {
		return nil
	}
	out := new(MaskinportenClientList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MaskinportenClientList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MaskinportenClientSpec) DeepCopyInto(out *MaskinportenClientSpec) {
	*out = *in
	if in.Scopes != nil {
		in, out := &in.Scopes, &out.Scopes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MaskinportenClientSpec.
func (in *MaskinportenClientSpec) DeepCopy() *MaskinportenClientSpec {
	if in == nil {
		return nil
	}
	out := new(MaskinportenClientSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MaskinportenClientStatus) DeepCopyInto(out *MaskinportenClientStatus) {
	*out = *in
	if in.KeyIds != nil {
		in, out := &in.KeyIds, &out.KeyIds
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.LastSynced != nil {
		in, out := &in.LastSynced, &out.LastSynced
		*out = (*in).DeepCopy()
	}
	if in.LastActions != nil {
		in, out := &in.LastActions, &out.LastActions
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MaskinportenClientStatus.
func (in *MaskinportenClientStatus) DeepCopy() *MaskinportenClientStatus {
	if in == nil {
		return nil
	}
	out := new(MaskinportenClientStatus)
	in.DeepCopyInto(out)
	return out
}
