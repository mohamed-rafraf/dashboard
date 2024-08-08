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

// TinkerbellHTTPSource TinkerbellHTTPSource represents list of images and their versions that can be downloaded over HTTP.
//
// swagger:model TinkerbellHTTPSource
type TinkerbellHTTPSource struct {

	// OperatingSystems represents list of supported operating-systems with their URLs.
	OperatingSystems map[string]OSVersions `json:"operatingSystems,omitempty"`
}

// Validate validates this tinkerbell HTTP source
func (m *TinkerbellHTTPSource) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateOperatingSystems(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *TinkerbellHTTPSource) validateOperatingSystems(formats strfmt.Registry) error {
	if swag.IsZero(m.OperatingSystems) { // not required
		return nil
	}

	for k := range m.OperatingSystems {

		if val, ok := m.OperatingSystems[k]; ok {
			if err := val.Validate(formats); err != nil {
				return err
			}
		}

	}

	return nil
}

// ContextValidate validate this tinkerbell HTTP source based on the context it is used
func (m *TinkerbellHTTPSource) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidateOperatingSystems(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *TinkerbellHTTPSource) contextValidateOperatingSystems(ctx context.Context, formats strfmt.Registry) error {

	for k := range m.OperatingSystems {

		if val, ok := m.OperatingSystems[k]; ok {
			if err := val.ContextValidate(ctx, formats); err != nil {
				return err
			}
		}

	}

	return nil
}

// MarshalBinary interface implementation
func (m *TinkerbellHTTPSource) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *TinkerbellHTTPSource) UnmarshalBinary(b []byte) error {
	var res TinkerbellHTTPSource
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
