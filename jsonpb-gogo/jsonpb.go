// Source is https://raw.githubusercontent.com/gogo/gateway/master/jsonpb.go.
// Required to resolve https://github.com/gogo/protobuf/issues/212.
package jsonpb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"github.com/gogo/protobuf/proto"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/zang-cloud/grpc-json/jsonpb"
)

// MarshalerGOGO is a runtime.Marshaler which marshals into
// JSON with "github.com/zang-cloud/grpc-json/jsonpb".
type MarshalerGOGO jsonpb.Marshaler

// ContentType always returns "application/json".
func (*MarshalerGOGO) ContentType() string {
	return "application/json"
}

// Marshal marshals "v" into JSON.
func (j *MarshalerGOGO) Marshal(w io.Writer, v interface{}) error {
	p, ok := v.(proto.Message)
	if !ok {
		buf, err := j.marshalNonProtoField(v)
		if err != nil {
			return err
		}
		_, err = w.Write(buf)
		return err
	}
	return (*jsonpb.Marshaler)(j).Marshal(w, p)
}

func (j *MarshalerGOGO) marshalV(v interface{}) ([]byte, error) {
	if _, ok := v.(proto.Message); !ok {
		return j.marshalNonProtoField(v)
	}
	var buf bytes.Buffer
	if err := j.Marshal(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// marshalNonProto marshals a non-message field of a protobuf message.
// This function does not correctly marshals arbitary data structure into JSON,
// but it is only capable of marshaling non-message field values of protobuf,
// i.e. primitive types, enums; pointers to primitives or enums; maps from
// integer/string types to primitives/enums/pointers to messages.
func (j *MarshalerGOGO) marshalNonProtoField(v interface{}) ([]byte, error) {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return []byte("null"), nil
		}
		rv = rv.Elem()
	}

	if rv.Kind() == reflect.Map {
		m := make(map[string]*json.RawMessage)
		for _, k := range rv.MapKeys() {
			buf, err := j.marshalV(rv.MapIndex(k).Interface())
			if err != nil {
				return nil, err
			}
			m[fmt.Sprintf("%v", k.Interface())] = (*json.RawMessage)(&buf)
		}
		if j.Indent != "" {
			return json.MarshalIndent(m, "", j.Indent)
		}
		return json.Marshal(m)
	}
	if enum, ok := rv.Interface().(protoEnum); ok && !j.EnumsAsInts {
		return json.Marshal(enum.String())
	}
	return json.Marshal(rv.Interface())
}

// NewEncoder returns an Encoder which writes JSON stream into "w".
func (j *MarshalerGOGO) NewEncoder(w io.Writer) runtime.Encoder {
	return runtime.EncoderFunc(func(v interface{}) error { return j.Marshal(w, v) })
}

// Delimiter for newline encoded JSON streams.
func (j *MarshalerGOGO) Delimiter() []byte {
	return []byte("\n")
}

// UnmarshalerGOGO is a runtime.Marshaler which unmarshals from
// JSON with "github.com/zang-cloud/grpc-json/jsonpb".
type UnmarshalerGOGO jsonpb.Unmarshaler

// Unmarshal unmarshals JSON "data" into "v".
// Currently it can only marshal proto.Message.
func (j *UnmarshalerGOGO) Unmarshal(r io.Reader, v interface{}) error {
	d := json.NewDecoder(r)
	return j.decodeJSONPb(d, v)
}

// NewDecoder returns a runtime.Decoder which reads JSON stream from "r".
func (j *UnmarshalerGOGO) NewDecoder(r io.Reader) runtime.Decoder {
	d := json.NewDecoder(r)
	return runtime.DecoderFunc(func(v interface{}) error { return j.decodeJSONPb(d, v) })
}

func (j *UnmarshalerGOGO) unmarshalJSONPb(data []byte, v interface{}) error {
	d := json.NewDecoder(bytes.NewReader(data))
	return j.decodeJSONPb(d, v)
}

func (j *UnmarshalerGOGO) decodeJSONPb(d *json.Decoder, v interface{}) error {
	p, ok := v.(proto.Message)
	if !ok {
		return j.decodeNonProtoField(d, v)
	}
	unmarshaler := (*jsonpb.Unmarshaler)(j)
	return unmarshaler.UnmarshalNext(d, p)
}

func (j *UnmarshalerGOGO) decodeNonProtoField(d *json.Decoder, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf("%T is not a pointer", v)
	}
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		if rv.Type().ConvertibleTo(typeProtoMessage) {
			unmarshaler := (*jsonpb.Unmarshaler)(j)
			return unmarshaler.UnmarshalNext(d, rv.Interface().(proto.Message))
		}
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Map {
		if rv.IsNil() {
			rv.Set(reflect.MakeMap(rv.Type()))
		}
		conv, ok := convFromType[rv.Type().Key().Kind()]
		if !ok {
			return fmt.Errorf("unsupported type of map field key: %v", rv.Type().Key())
		}
		m := make(map[string]*json.RawMessage)
		if err := d.Decode(&m); err != nil {
			return err
		}
		for k, v := range m {
			result := conv.Call([]reflect.Value{reflect.ValueOf(k)})
			if err := result[1].Interface(); err != nil {
				return err.(error)
			}
			bk := result[0]
			bv := reflect.New(rv.Type().Elem())
			if err := j.unmarshalJSONPb([]byte(*v), bv.Interface()); err != nil {
				return err
			}
			rv.SetMapIndex(bk, bv.Elem())
		}
		return nil
	}
	if _, ok := rv.Interface().(protoEnum); ok {
		var repr interface{}
		if err := d.Decode(&repr); err != nil {
			return err
		}
		switch repr.(type) {
		case string:
			return fmt.Errorf("unmarshaling of symbolic enum %q not supported: %T", repr, rv.Interface())
		case float64:
			rv.Set(reflect.ValueOf(int32(repr.(float64))).Convert(rv.Type()))
			return nil
		default:
			return fmt.Errorf("cannot assign %#v into Go type %T", repr, rv.Interface())
		}
	}
	return d.Decode(v)
}

type protoEnum interface {
	fmt.Stringer
	EnumDescriptor() ([]byte, []int)
}

var typeProtoMessage = reflect.TypeOf((*proto.Message)(nil)).Elem()

var convFromType = map[reflect.Kind]reflect.Value{
	reflect.String:  reflect.ValueOf(runtime.String),
	reflect.Bool:    reflect.ValueOf(runtime.Bool),
	reflect.Float64: reflect.ValueOf(runtime.Float64),
	reflect.Float32: reflect.ValueOf(runtime.Float32),
	reflect.Int64:   reflect.ValueOf(runtime.Int64),
	reflect.Int32:   reflect.ValueOf(runtime.Int32),
	reflect.Uint64:  reflect.ValueOf(runtime.Uint64),
	reflect.Uint32:  reflect.ValueOf(runtime.Uint32),
	reflect.Slice:   reflect.ValueOf(runtime.Bytes),
}