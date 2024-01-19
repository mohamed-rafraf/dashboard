// Code generated by go-swagger; DO NOT EDIT.

package anexia

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"k8c.io/dashboard/v2/pkg/test/e2e/utils/apiclient/models"
)

// ListAnexiaDiskTypesNoCredentialsV2Reader is a Reader for the ListAnexiaDiskTypesNoCredentialsV2 structure.
type ListAnexiaDiskTypesNoCredentialsV2Reader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *ListAnexiaDiskTypesNoCredentialsV2Reader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewListAnexiaDiskTypesNoCredentialsV2OK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewListAnexiaDiskTypesNoCredentialsV2Default(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewListAnexiaDiskTypesNoCredentialsV2OK creates a ListAnexiaDiskTypesNoCredentialsV2OK with default headers values
func NewListAnexiaDiskTypesNoCredentialsV2OK() *ListAnexiaDiskTypesNoCredentialsV2OK {
	return &ListAnexiaDiskTypesNoCredentialsV2OK{}
}

/*
ListAnexiaDiskTypesNoCredentialsV2OK describes a response with status code 200, with default header values.

AnexiaDiskTypeList
*/
type ListAnexiaDiskTypesNoCredentialsV2OK struct {
	Payload models.AnexiaDiskTypeList
}

// IsSuccess returns true when this list anexia disk types no credentials v2 o k response has a 2xx status code
func (o *ListAnexiaDiskTypesNoCredentialsV2OK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this list anexia disk types no credentials v2 o k response has a 3xx status code
func (o *ListAnexiaDiskTypesNoCredentialsV2OK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this list anexia disk types no credentials v2 o k response has a 4xx status code
func (o *ListAnexiaDiskTypesNoCredentialsV2OK) IsClientError() bool {
	return false
}

// IsServerError returns true when this list anexia disk types no credentials v2 o k response has a 5xx status code
func (o *ListAnexiaDiskTypesNoCredentialsV2OK) IsServerError() bool {
	return false
}

// IsCode returns true when this list anexia disk types no credentials v2 o k response a status code equal to that given
func (o *ListAnexiaDiskTypesNoCredentialsV2OK) IsCode(code int) bool {
	return code == 200
}

func (o *ListAnexiaDiskTypesNoCredentialsV2OK) Error() string {
	return fmt.Sprintf("[GET /api/v2/projects/{project_id}/clusters/{cluster_id}/providers/anexia/disk-types][%d] listAnexiaDiskTypesNoCredentialsV2OK  %+v", 200, o.Payload)
}

func (o *ListAnexiaDiskTypesNoCredentialsV2OK) String() string {
	return fmt.Sprintf("[GET /api/v2/projects/{project_id}/clusters/{cluster_id}/providers/anexia/disk-types][%d] listAnexiaDiskTypesNoCredentialsV2OK  %+v", 200, o.Payload)
}

func (o *ListAnexiaDiskTypesNoCredentialsV2OK) GetPayload() models.AnexiaDiskTypeList {
	return o.Payload
}

func (o *ListAnexiaDiskTypesNoCredentialsV2OK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewListAnexiaDiskTypesNoCredentialsV2Default creates a ListAnexiaDiskTypesNoCredentialsV2Default with default headers values
func NewListAnexiaDiskTypesNoCredentialsV2Default(code int) *ListAnexiaDiskTypesNoCredentialsV2Default {
	return &ListAnexiaDiskTypesNoCredentialsV2Default{
		_statusCode: code,
	}
}

/*
ListAnexiaDiskTypesNoCredentialsV2Default describes a response with status code -1, with default header values.

errorResponse
*/
type ListAnexiaDiskTypesNoCredentialsV2Default struct {
	_statusCode int

	Payload *models.ErrorResponse
}

// Code gets the status code for the list anexia disk types no credentials v2 default response
func (o *ListAnexiaDiskTypesNoCredentialsV2Default) Code() int {
	return o._statusCode
}

// IsSuccess returns true when this list anexia disk types no credentials v2 default response has a 2xx status code
func (o *ListAnexiaDiskTypesNoCredentialsV2Default) IsSuccess() bool {
	return o._statusCode/100 == 2
}

// IsRedirect returns true when this list anexia disk types no credentials v2 default response has a 3xx status code
func (o *ListAnexiaDiskTypesNoCredentialsV2Default) IsRedirect() bool {
	return o._statusCode/100 == 3
}

// IsClientError returns true when this list anexia disk types no credentials v2 default response has a 4xx status code
func (o *ListAnexiaDiskTypesNoCredentialsV2Default) IsClientError() bool {
	return o._statusCode/100 == 4
}

// IsServerError returns true when this list anexia disk types no credentials v2 default response has a 5xx status code
func (o *ListAnexiaDiskTypesNoCredentialsV2Default) IsServerError() bool {
	return o._statusCode/100 == 5
}

// IsCode returns true when this list anexia disk types no credentials v2 default response a status code equal to that given
func (o *ListAnexiaDiskTypesNoCredentialsV2Default) IsCode(code int) bool {
	return o._statusCode == code
}

func (o *ListAnexiaDiskTypesNoCredentialsV2Default) Error() string {
	return fmt.Sprintf("[GET /api/v2/projects/{project_id}/clusters/{cluster_id}/providers/anexia/disk-types][%d] listAnexiaDiskTypesNoCredentialsV2 default  %+v", o._statusCode, o.Payload)
}

func (o *ListAnexiaDiskTypesNoCredentialsV2Default) String() string {
	return fmt.Sprintf("[GET /api/v2/projects/{project_id}/clusters/{cluster_id}/providers/anexia/disk-types][%d] listAnexiaDiskTypesNoCredentialsV2 default  %+v", o._statusCode, o.Payload)
}

func (o *ListAnexiaDiskTypesNoCredentialsV2Default) GetPayload() *models.ErrorResponse {
	return o.Payload
}

func (o *ListAnexiaDiskTypesNoCredentialsV2Default) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ErrorResponse)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
