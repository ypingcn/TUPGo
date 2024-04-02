package tup

import (
	"reflect"

	"github.com/TarsCloud/TarsGo/tars/protocol/codec"
	"github.com/TarsCloud/TarsGo/tars/util/tools"
)

// PacketBuffer store data
type PacketBuffer struct {
	IVersion int16
	Data     map[string]map[string][]byte // for version 2
	NewData  map[string][]byte            // for version 3
	iRet     int32                        // function ret in response
}

// GetRawData get raw data
func (pb *PacketBuffer) GetRawData() (interface{}, error) {
	if pb.IVersion == 2 {
		data := make(map[string]map[string][]byte)
		for k, v := range pb.Data {
			data[k] = make(map[string][]byte)
			for k1, v1 := range v {
				data[k][k1] = make([]byte, len(v1))
				copy(data[k][k1], v1)
			}
		}
		return data, nil
	} else if pb.IVersion == 3 {
		newData := make(map[string][]byte)
		for k, v := range pb.NewData {
			newData[k] = make([]byte, len(v))
			copy(newData[k], v)
		}
		return newData, nil
	}
	return nil, ErrTUPVersionNotSupported
}

func (pb *PacketBuffer) get(key string, value interface{}) error {
	if pb.IVersion != 2 && pb.IVersion != 3 {
		return ErrTUPVersionNotSupported
	}
	if key == "" {
		value = pb.iRet
		return nil
	}

	if pb.IVersion == 2 {
		className := reflect.TypeOf(value).Elem().String()
		if val, ok := pb.Data[key]; ok {
			if val1, ok1 := val[className]; ok1 {
				is := codec.NewReader(val1)
				receiver := reflect.ValueOf(value)
				arg := reflect.ValueOf(is)
				f, have := reflect.TypeOf(value).MethodByName("ReadFrom")
				if have {
					f.Func.Call([]reflect.Value{receiver, arg})
				} else {
					return ErrNeedReadFrom
				}
			}
		}
	} else if pb.IVersion == 3 {
		if val, ok := pb.NewData[key]; ok {
			is := codec.NewReader(val)
			receiver := reflect.ValueOf(value)
			arg := reflect.ValueOf(is)
			f, have := reflect.TypeOf(value).MethodByName("ReadFrom")
			if have {
				f.Func.Call([]reflect.Value{receiver, arg})
			} else {
				return ErrNeedReadFrom
			}
		}
	}
	return nil
}

func (pb *PacketBuffer) put(key string, value interface{}) error {
	if pb.IVersion != 2 && pb.IVersion != 3 {
		return ErrTUPVersionNotSupported
	}
	os := codec.NewBuffer()
	receiver := reflect.ValueOf(value)
	arg := reflect.ValueOf(os)
	f, have := reflect.TypeOf(value).MethodByName("WriteTo")
	if !have {
		return ErrNeedWriteTo
	}

	f.Func.Call([]reflect.Value{receiver, arg})
	bs := os.ToBytes()

	if pb.IVersion == 2 {
		className := reflect.TypeOf(value).Elem().String()
		if pb.Data[key] == nil {
			pb.Data[key] = make(map[string][]byte)
		}
		if pb.Data[key][className] == nil {
			pb.Data[key][className] = make([]byte, len(bs))
		}
		pb.Data[key][className] = bs
	} else if pb.IVersion == 3 {
		pb.NewData[key] = make([]byte, len(bs))
		pb.NewData[key] = bs
	}
	return nil
}

func (pb *PacketBuffer) writeTo(os *codec.Buffer) error {
	var err error
	os.Reset()
	if pb.IVersion == 2 {
		err = os.WriteHead(codec.MAP, 0)
		if err != nil {
			return err
		}
		err = os.WriteInt32(int32(len(pb.Data)), 0)
		if err != nil {
			return err
		}
		for k, v := range pb.Data {
			err = os.WriteString(k, 0)
			if err != nil {
				return err
			}
			err = os.WriteHead(codec.MAP, 1)
			if err != nil {
				return err
			}
			err = os.WriteInt32(int32(len(v)), 0)
			if err != nil {
				return err
			}
			for className, value := range v {
				err = os.WriteString(className, 0)
				if err != nil {
					return err
				}
				err = os.WriteHead(codec.SimpleList, 1)
				if err != nil {
					return err
				}
				err = os.WriteHead(codec.BYTE, 0)
				if err != nil {
					return err
				}
				err = os.WriteInt32(int32(len(value)+2), 0)
				if err != nil {
					return err
				}
				err = os.WriteHead(codec.StructBegin, 0)
				if err != nil {
					return err
				}
				err = os.WriteSliceInt8(tools.ByteToInt8(value))
				if err != nil {
					return err
				}
				err = os.WriteHead(codec.StructEnd, 0)
				if err != nil {
					return err
				}
			}
		}
	} else if pb.IVersion == 3 {
		err = os.WriteHead(codec.MAP, 0)
		if err != nil {
			return err
		}
		err = os.WriteInt32(int32(len(pb.NewData)), 0)
		if err != nil {
			return err
		}
		for k, v := range pb.NewData {
			err = os.WriteString(k, 0)
			if err != nil {
				return err
			}
			err = os.WriteHead(codec.SimpleList, 1)
			if err != nil {
				return err
			}

			err = os.WriteHead(codec.BYTE, 0)
			if err != nil {
				return err
			}
			err = os.WriteInt32(int32(len(v)+2), 0) // struct begin + struct end
			if err != nil {
				return err
			}
			err = os.WriteHead(codec.StructBegin, 0)
			if err != nil {
				return err
			}
			err = os.WriteSliceInt8(tools.ByteToInt8(v))
			if err != nil {
				return err
			}
			err = os.WriteHead(codec.StructEnd, 0)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (pb *PacketBuffer) readFrom(is *codec.Reader) error {
	var err error
	var length int32

	if pb.IVersion == 2 {
		_, err = is.SkipTo(codec.MAP, 0, false)
		if err != nil {
			return err
		}
		err = is.ReadInt32(&length, 0, false)
		if err != nil {
			return err
		}

		for i1, e1 := int32(0), length; i1 < e1; i1++ {
			var k1 string

			err = is.ReadString(&k1, 0, false)
			if err != nil {
				return err
			}

			if k1 == "" { // for func ret
				_, err = is.SkipTo(codec.MAP, 1, false)
				if err != nil {
					return err
				}
				var mapLength int32
				err = is.ReadInt32(&mapLength, 0, false)
				if err != nil {
					return err
				}

				for i2, e2 := int32(0), mapLength; i2 < e2; i2++ {
					var k2 string // int32
					err = is.ReadString(&k2, 0, false)
					if err != nil {
						return err
					}

					var retLength int32
					_, err = is.SkipTo(codec.SimpleList, 1, false)
					if err != nil {
						return err
					}
					_, err = is.SkipTo(codec.BYTE, 0, false)
					if err != nil {
						return err
					}
					err := is.ReadInt32(&retLength, 0, false)
					if err != nil {
						return err
					}
					err = is.ReadInt32(&pb.iRet, 0, false)
					if err != nil {
						return err
					}
				}
			} else {
				_, err = is.SkipTo(codec.MAP, 1, false)
				if err != nil {
					return err
				}
				var mapLength int32
				err = is.ReadInt32(&mapLength, 0, false)
				if err != nil {
					return err
				}

				for i2, e2 := int32(0), mapLength; i2 < e2; i2++ {
					var k2 string
					var v []int8
					var v1 []byte

					err = is.ReadString(&k2, 0, false)
					if err != nil {
						return err
					}

					_, err = is.SkipTo(codec.SimpleList, 1, false)
					if err != nil {
						return err
					}

					_, err = is.SkipTo(codec.BYTE, 0, false)
					if err != nil {
						return err
					}

					var byteLength int32

					err = is.ReadInt32(&byteLength, 0, false)
					if err != nil {
						return err
					}

					_, err = is.SkipTo(codec.StructBegin, 0, false)
					if err != nil {
						return err
					}

					err = is.ReadSliceInt8(&v, byteLength, false)
					if err != nil {
						return err
					}

					v1 = tools.Int8ToByte(v)

					if pb.Data[k1] == nil {
						pb.Data[k1] = make(map[string][]byte)
					}
					if pb.Data[k1][k2] == nil {
						pb.Data[k1][k2] = make([]byte, byteLength)
					}
					pb.Data[k1][k2] = v1
				}
			}
		}

	} else if pb.IVersion == 3 {
		_, err = is.SkipTo(codec.MAP, 0, false)
		if err != nil {
			return err
		}
		err = is.ReadInt32(&length, 0, false)
		if err != nil {
			return err
		}

		for i1, e1 := int32(0), length; i1 < e1; i1++ {
			var k1 string
			var v []int8
			var v1 []byte

			err = is.ReadString(&k1, 0, false)
			if err != nil {
				return err
			}

			if k1 == "" { // for func ret
				var retLength int32
				_, err = is.SkipTo(codec.SimpleList, 1, false)
				if err != nil {
					return err
				}
				_, err = is.SkipTo(codec.BYTE, 0, false)
				if err != nil {
					return err
				}
				err := is.ReadInt32(&retLength, 0, false)
				if err != nil {
					return err
				}

				err = is.ReadInt32(&pb.iRet, 0, false)
				if err != nil {
					return err
				}
			} else {
				_, err = is.SkipTo(codec.SimpleList, 1, false)
				if err != nil {
					return err
				}

				_, err = is.SkipTo(codec.BYTE, 0, false)
				if err != nil {
					return err
				}

				var byteLength int32

				err = is.ReadInt32(&byteLength, 0, false)
				if err != nil {
					return err
				}

				_, err = is.SkipTo(codec.StructBegin, 0, false)
				if err != nil {
					return err
				}

				err = is.ReadSliceInt8(&v, byteLength, false)
				if err != nil {
					return err
				}

				v1 = tools.Int8ToByte(v)

				if pb.NewData[k1] == nil {
					pb.NewData[k1] = make([]byte, byteLength)
				}
				pb.NewData[k1] = v1
			}

		}
	}
	return nil
}
