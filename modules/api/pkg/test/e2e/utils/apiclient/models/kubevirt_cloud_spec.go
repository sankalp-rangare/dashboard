// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"
	"strconv"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
)

// KubevirtCloudSpec KubevirtCloudSpec specifies the access data to Kubevirt.
//
// swagger:model KubevirtCloudSpec
type KubevirtCloudSpec struct {

	// c s i kubeconfig
	CSIKubeconfig string `json:"csiKubeconfig,omitempty"`

	// InfraStorageClasses is a list of storage classes from KubeVirt infra cluster that are used for
	// initialization of user cluster storage classes by the CSI driver kubevirt (hot pluggable disks)
	InfraStorageClasses []string `json:"infraStorageClasses"`

	// The cluster's kubeconfig file, encoded with base64.
	Kubeconfig string `json:"kubeconfig,omitempty"`

	// PreAllocatedDataVolumes holds list of preallocated DataVolumes which can be used as reference for DataVolume cloning.
	// PreAllocatedDataVolumes represents custom-images tied to cluster and managed by user.
	PreAllocatedDataVolumes []*PreAllocatedDataVolume `json:"preAllocatedDataVolumes"`

	// credentials reference
	CredentialsReference *GlobalSecretKeySelector `json:"credentialsReference,omitempty"`
}

// Validate validates this kubevirt cloud spec
func (m *KubevirtCloudSpec) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validatePreAllocatedDataVolumes(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateCredentialsReference(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *KubevirtCloudSpec) validatePreAllocatedDataVolumes(formats strfmt.Registry) error {
	if swag.IsZero(m.PreAllocatedDataVolumes) { // not required
		return nil
	}

	for i := 0; i < len(m.PreAllocatedDataVolumes); i++ {
		if swag.IsZero(m.PreAllocatedDataVolumes[i]) { // not required
			continue
		}

		if m.PreAllocatedDataVolumes[i] != nil {
			if err := m.PreAllocatedDataVolumes[i].Validate(formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("preAllocatedDataVolumes" + "." + strconv.Itoa(i))
				} else if ce, ok := err.(*errors.CompositeError); ok {
					return ce.ValidateName("preAllocatedDataVolumes" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

func (m *KubevirtCloudSpec) validateCredentialsReference(formats strfmt.Registry) error {
	if swag.IsZero(m.CredentialsReference) { // not required
		return nil
	}

	if m.CredentialsReference != nil {
		if err := m.CredentialsReference.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("credentialsReference")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("credentialsReference")
			}
			return err
		}
	}

	return nil
}

// ContextValidate validate this kubevirt cloud spec based on the context it is used
func (m *KubevirtCloudSpec) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidatePreAllocatedDataVolumes(ctx, formats); err != nil {
		res = append(res, err)
	}

	if err := m.contextValidateCredentialsReference(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *KubevirtCloudSpec) contextValidatePreAllocatedDataVolumes(ctx context.Context, formats strfmt.Registry) error {

	for i := 0; i < len(m.PreAllocatedDataVolumes); i++ {

		if m.PreAllocatedDataVolumes[i] != nil {
			if err := m.PreAllocatedDataVolumes[i].ContextValidate(ctx, formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("preAllocatedDataVolumes" + "." + strconv.Itoa(i))
				} else if ce, ok := err.(*errors.CompositeError); ok {
					return ce.ValidateName("preAllocatedDataVolumes" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

func (m *KubevirtCloudSpec) contextValidateCredentialsReference(ctx context.Context, formats strfmt.Registry) error {

	if m.CredentialsReference != nil {
		if err := m.CredentialsReference.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("credentialsReference")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("credentialsReference")
			}
			return err
		}
	}

	return nil
}

// MarshalBinary interface implementation
func (m *KubevirtCloudSpec) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *KubevirtCloudSpec) UnmarshalBinary(b []byte) error {
	var res KubevirtCloudSpec
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
