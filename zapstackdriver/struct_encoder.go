package zapstackdriver

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/struct"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

// listValueEncoder wrapped structpb., implements zapcore.ArrayEncoder
type listValueEncoder struct {
	*structpb.ListValue
}

func newListValueEncoder() *listValueEncoder {
	return &listValueEncoder{
		ListValue: &structpb.ListValue{
			Values: []*structpb.Value{},
		},
	}
}

func (l *listValueEncoder) appendFloat64(v float64) {
	l.ListValue.Values = append(l.ListValue.Values, &structpb.Value{
		Kind: &structpb.Value_NumberValue{
			NumberValue: float64(v),
		},
	})
}

// AppendBool -
func (l *listValueEncoder) AppendBool(v bool) {
	l.ListValue.Values = append(l.ListValue.Values, &structpb.Value{
		Kind: &structpb.Value_BoolValue{
			BoolValue: v,
		},
	})
}

// AppendByteString -
func (l *listValueEncoder) AppendByteString(v []byte) {
	l.ListValue.Values = append(l.ListValue.Values, &structpb.Value{
		Kind: &structpb.Value_StringValue{
			StringValue: string(v),
		},
	})
}

// AppendComplex128 -
func (l *listValueEncoder) AppendComplex128(v complex128) {
	l.ListValue.Values = append(l.ListValue.Values, &structpb.Value{
		Kind: &structpb.Value_StringValue{
			StringValue: fmt.Sprint(v),
		},
	})
}

// AppendComplex64 -
func (l *listValueEncoder) AppendComplex64(v complex64) {
	l.ListValue.Values = append(l.ListValue.Values, &structpb.Value{
		Kind: &structpb.Value_StringValue{
			StringValue: fmt.Sprint(v),
		},
	})
}

// AppendFloat64 -
func (l *listValueEncoder) AppendFloat64(v float64) {
	l.appendFloat64(v)
}

// AppendFloat32 -
func (l *listValueEncoder) AppendFloat32(v float32) {
	l.appendFloat64(float64(v))
}

// AppendInt -
func (l *listValueEncoder) AppendInt(v int) {
	l.appendFloat64(float64(v))
}

// AppendInt64 -
func (l *listValueEncoder) AppendInt64(v int64) {
	l.appendFloat64(float64(v))

}

// AppendInt32 -
func (l *listValueEncoder) AppendInt32(v int32) {
	l.appendFloat64(float64(v))

}

// AppendInt16 -
func (l *listValueEncoder) AppendInt16(v int16) {
	l.appendFloat64(float64(v))

}

// AppendInt8 -
func (l *listValueEncoder) AppendInt8(v int8) {
	l.appendFloat64(float64(v))

}

// AppendString -
func (l *listValueEncoder) AppendString(v string) {
	l.ListValue.Values = append(l.ListValue.Values, &structpb.Value{
		Kind: &structpb.Value_StringValue{
			StringValue: v,
		},
	})
}

// AppendUint -
func (l *listValueEncoder) AppendUint(v uint) {
	l.appendFloat64(float64(v))

}

// AppendUint64 -
func (l *listValueEncoder) AppendUint64(v uint64) {
	l.appendFloat64(float64(v))

}

// AppendUint32 -
func (l *listValueEncoder) AppendUint32(v uint32) {
	l.appendFloat64(float64(v))

}

// AppendUint16 -
func (l *listValueEncoder) AppendUint16(v uint16) {
	l.appendFloat64(float64(v))

}

// AppendUint8 -
func (l *listValueEncoder) AppendUint8(v uint8) {
	l.appendFloat64(float64(v))

}

// AppendUintptr -
func (l *listValueEncoder) AppendUintptr(v uintptr) {
	l.appendFloat64(float64(v))
}

// Time-related types.
func (l *listValueEncoder) AppendDuration(v time.Duration) {
	l.appendFloat64(float64(v))
}

func (l *listValueEncoder) AppendTime(v time.Time) {
	l.appendFloat64(float64(v.UnixNano()))
}

// Logging-specific marshalers.
func (l *listValueEncoder) AppendArray(v zapcore.ArrayMarshaler) error {
	al := newListValueEncoder()
	err := v.MarshalLogArray(al)
	if err != nil {
		return err
	}

	l.ListValue.Values = append(l.ListValue.Values, &structpb.Value{
		Kind: &structpb.Value_ListValue{
			ListValue: al.ListValue,
		},
	})
	return nil
}

func (l *listValueEncoder) AppendObject(zapcore.ObjectMarshaler) error {
	return nil
}

// AppendReflected uses reflection to serialize arbitrary objects, so it's
// slow and allocation-heavy.
func (l *listValueEncoder) AppendReflected(v interface{}) error {
	return nil
}

// StructEncoder implements zapcore.ObjectEncoder that will encode log into
// https://github.com/golang/protobuf/blob/master/ptypes/struct/struct.proto
type StructEncoder struct {
	*structpb.Struct
}

// NewStructEncoder -
func NewStructEncoder() *StructEncoder {
	return &StructEncoder{
		Struct: &structpb.Struct{
			Fields: map[string]*structpb.Value{},
		},
	}
}

// AddArray -
func (e *StructEncoder) AddArray(key string, v zapcore.ArrayMarshaler) error {
	enc := newListValueEncoder()
	err := v.MarshalLogArray(enc)
	if err != nil {
		return err
	}
	e.Fields[key] = &structpb.Value{
		Kind: &structpb.Value_ListValue{
			ListValue: enc.ListValue,
		},
	}
	return nil
}

// AddObject -
func (e *StructEncoder) AddObject(key string, v zapcore.ObjectMarshaler) error {
	enc := NewStructEncoder()
	err := v.MarshalLogObject(enc)
	if err != nil {
		return err
	}
	e.Fields[key] = &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: enc.Struct,
		},
	}
	return nil
}

// AddBinary -
func (e *StructEncoder) AddBinary(key string, v []byte) {
	e.AddString(key, base64.StdEncoding.EncodeToString(v))
}

// AddByteString -
func (e *StructEncoder) AddByteString(key string, v []byte) { e.AddString(key, string(v)) }

// AddBool -
func (e *StructEncoder) AddBool(key string, v bool) {
	e.Fields[key] = &structpb.Value{
		Kind: &structpb.Value_BoolValue{
			BoolValue: v,
		},
	}
}

// AddComplex128 -
func (e *StructEncoder) AddComplex128(key string, v complex128) { e.AddString(key, fmt.Sprint(v)) }

// AddComplex64 -
func (e *StructEncoder) AddComplex64(key string, v complex64) { e.AddString(key, fmt.Sprint(v)) }

// AddDuration -
func (e *StructEncoder) AddDuration(key string, v time.Duration) { e.AddFloat64(key, float64(v)) }

// AddFloat64 -
func (e *StructEncoder) AddFloat64(key string, v float64) {
	e.Fields[key] = &structpb.Value{
		Kind: &structpb.Value_NumberValue{
			NumberValue: v,
		},
	}
}

// AddFloat32 -
func (e *StructEncoder) AddFloat32(key string, v float32) { e.AddFloat64(key, float64(v)) }

// AddInt -
func (e *StructEncoder) AddInt(key string, v int) { e.AddFloat64(key, float64(v)) }

// AddInt64 -
func (e *StructEncoder) AddInt64(key string, v int64) { e.AddFloat64(key, float64(v)) }

// AddInt32 -
func (e *StructEncoder) AddInt32(key string, v int32) { e.AddFloat64(key, float64(v)) }

// AddInt16 -
func (e *StructEncoder) AddInt16(key string, v int16) { e.AddFloat64(key, float64(v)) }

// AddInt8 -
func (e *StructEncoder) AddInt8(key string, v int8) { e.AddFloat64(key, float64(v)) }

// AddString -
func (e *StructEncoder) AddString(key, v string) {
	e.Fields[key] = &structpb.Value{
		Kind: &structpb.Value_StringValue{
			StringValue: v,
		},
	}
}

// AddTime -
func (e *StructEncoder) AddTime(key string, v time.Time) { e.AddFloat64(key, float64(v.UnixNano())) }

// AddUint -
func (e *StructEncoder) AddUint(key string, v uint) { e.AddFloat64(key, float64(v)) }

// AddUint64 -
func (e *StructEncoder) AddUint64(key string, v uint64) { e.AddFloat64(key, float64(v)) }

// AddUint32 -
func (e *StructEncoder) AddUint32(key string, v uint32) { e.AddFloat64(key, float64(v)) }

// AddUint16 -
func (e *StructEncoder) AddUint16(key string, v uint16) { e.AddFloat64(key, float64(v)) }

// AddUint8 -
func (e *StructEncoder) AddUint8(key string, v uint8) { e.AddFloat64(key, float64(v)) }

// AddUintptr -
func (e *StructEncoder) AddUintptr(key string, v uintptr) { e.AddUint64(key, uint64(v)) }

// AddReflected uses reflection to serialize arbitrary objects, so it's slow
// and allocation-heavy.
func (e *StructEncoder) AddReflected(key string, v interface{}) error {
	marshaled, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var m map[string]interface{}
	err = json.Unmarshal(marshaled, &m)
	if err != nil {
		return err
	}
	st, err := jsonMapToProtoStruct(m)
	if err != nil {
		return err
	}
	e.Fields[key] = &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: st,
		},
	}
	return nil
}

func jsonMapToProtoStruct(m map[string]interface{}) (*structpb.Struct, error) {
	fields := map[string]*structpb.Value{}
	for k, v := range m {
		sv, err := jsonValueToStructValue(v)
		if err != nil {
			return nil, err
		}
		fields[k] = sv
	}
	return &structpb.Struct{Fields: fields}, nil
}

func jsonValueToStructValue(v interface{}) (*structpb.Value, error) {
	switch x := v.(type) {
	case bool:
		return &structpb.Value{Kind: &structpb.Value_BoolValue{BoolValue: x}}, nil
	case float64:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: x}}, nil
	case string:
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: x}}, nil
	case nil:
		return &structpb.Value{Kind: &structpb.Value_NullValue{}}, nil
	case map[string]interface{}:
		sv, err := jsonMapToProtoStruct(x)
		if err != nil {
			return nil, err
		}
		return &structpb.Value{Kind: &structpb.Value_StructValue{StructValue: sv}}, nil
	case []interface{}:
		var vals []*structpb.Value
		for _, e := range x {
			sv, err := jsonValueToStructValue(e)
			if err != nil {
				return nil, err
			}
			vals = append(vals, sv)
		}
		return &structpb.Value{Kind: &structpb.Value_ListValue{ListValue: &structpb.ListValue{Values: vals}}}, nil
	default:
		return nil, fmt.Errorf("bad type %T for JSON value", v)
	}
}

// OpenNamespace opens an isolated namespace where all subsequent fields will
// be added. Applications can use namespaces to prevent key collisions when
// injecting loggers into sub-components or third-party libraries.
func (e *StructEncoder) OpenNamespace(key string) {

}

// Clone -
func (e *StructEncoder) Clone() zapcore.Encoder {
	return e.clone()
}

func (e *StructEncoder) clone() *StructEncoder {
	ce := proto.Clone(e.Struct).(*structpb.Struct)
	if ce.Fields == nil {
		ce.Fields = map[string]*structpb.Value{}
	}
	return &StructEncoder{Struct: ce}
}

// EncodeEntry -
func (e *StructEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	return nil, errors.New("NOT IMPLEMENTED")
}

func (e *StructEncoder) encodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*StructEncoder, error) {
	e2 := e.clone()
	addFields(e2, fields)
	return e2, nil
}

func addFields(enc zapcore.ObjectEncoder, fields []zapcore.Field) {
	for i := range fields {
		fields[i].AddTo(enc)
	}
}
