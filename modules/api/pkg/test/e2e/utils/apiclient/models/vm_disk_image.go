// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
)

// VMDiskImage VMDiskImage represents a source of standard VM Image.
//
// swagger:model VMDiskImage
type VMDiskImage struct {

	// images
	Images map[string]OSVersions `json:"images,omitempty"`

	// source
	Source string `json:"source,omitempty"`
}

// Validate validates this VM disk image
func (m *VMDiskImage) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateImages(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *VMDiskImage) validateImages(formats strfmt.Registry) error {
	if swag.IsZero(m.Images) { // not required
		return nil
	}

	for k := range m.Images {

		if val, ok := m.Images[k]; ok {
			if err := val.Validate(formats); err != nil {
				return err
			}
		}

	}

	return nil
}

// ContextValidate validate this VM disk image based on the context it is used
func (m *VMDiskImage) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidateImages(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *VMDiskImage) contextValidateImages(ctx context.Context, formats strfmt.Registry) error {

	for k := range m.Images {

		if val, ok := m.Images[k]; ok {
			if err := val.ContextValidate(ctx, formats); err != nil {
				return err
			}
		}

	}

	return nil
}

// MarshalBinary interface implementation
func (m *VMDiskImage) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *VMDiskImage) UnmarshalBinary(b []byte) error {
	var res VMDiskImage
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
