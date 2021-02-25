package conversion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

/*

FailedMessage:
Response is Success if empty string:
"result": {
  "status": "Success"
},
Response is Failed:
"result": {
  "status": "Failed",
  "message": FailedMessage
}

ConvertedObjects:
# Objects must match the order of request.objects, and have apiVersion set to <request.desiredAPIVersion>.
# kind, metadata.uid, metadata.name, and metadata.namespace fields must not be changed by the webhook.
# metadata.labels and metadata.annotations fields may be changed by the webhook.
# All other changes to metadata fields by the webhook are ignored.
*/
type Response struct {
	FailedMessage    string                      `json:"failedMessage"`
	ConvertedObjects []unstructured.Unstructured `json:"convertedObjects,omitempty"`
}

func ResponseFromFile(filePath string) (*Response, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	if len(data) == 0 {
		return nil, nil
	}
	return ResponseFromBytes(data)
}

func ResponseFromBytes(data []byte) (*Response, error) {
	return ResponseFromReader(bytes.NewReader(data))
}

func ResponseFromReader(r io.Reader) (*Response, error) {
	response := new(Response)

	dec := json.NewDecoder(r)

	err := dec.Decode(response)

	if err != nil {
		return nil, err
	}

	return response, nil
}

func (r *Response) Dump() string {
	b := new(strings.Builder)
	b.WriteString("conversion.Response(")
	if r.FailedMessage != "" {
		b.WriteString("failedMessage=")
		b.WriteString(r.FailedMessage)
	}
	if len(r.ConvertedObjects) > 0 {
		if r.FailedMessage != "" {
			b.WriteRune(',')
		}
		b.WriteString("convertedObjects.len=")
		b.WriteString(strconv.FormatInt(int64(len(r.ConvertedObjects)), 10))
	}
	b.WriteString(")")
	return b.String()
}
