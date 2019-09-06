package codec

import (
	"encoding/json"
	"io"
	"reflect"

	"github.com/funny/link"
)

type JsonProtocol struct {
	types map[string]reflect.Type  //key-类型
	names map[reflect.Type]string //类型-key
}

//json协议创建
func Json() *JsonProtocol {
	return &JsonProtocol{
		types: make(map[string]reflect.Type),
		names: make(map[reflect.Type]string),
	}
}

//注册类型
func (j *JsonProtocol) Register(t interface{}) {

	//全部成员字段信息
	rt := reflect.TypeOf(t)
	//获取目标类型
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	//包路径+类型的名字
	name := rt.PkgPath() + "/" + rt.Name()
	j.types[name] = rt
	j.names[rt] = name
}

//通过名字注册
func (j *JsonProtocol) RegisterName(name string, t interface{}) {
	rt := reflect.TypeOf(t)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	j.types[name] = rt
	j.names[rt] = name
}

//定义一个codec解析器
func (j *JsonProtocol) NewCodec(rw io.ReadWriter) (link.Codec, error) {
	codec := &jsonCodec{
		p:       j,
		encoder: json.NewEncoder(rw),
		decoder: json.NewDecoder(rw),
	}
	//实现了关闭接口
	codec.closer, _ = rw.(io.Closer)
	return codec, nil
}

type jsonIn struct {
	Head string //头部数据
	Body *json.RawMessage //原生消息
}

//返回数据
type jsonOut struct {
	Head string //头部
	Body interface{}. //bodys
}

type jsonCodec struct {
	p       *JsonProtocol //类型注册器
	closer  io.Closer
	encoder *json.Encoder
	decoder *json.Decoder
}

//解码
func (c *jsonCodec) Receive() (interface{}, error) {
	var in jsonIn
	err := c.decoder.Decode(&in)
	if err != nil {
		return nil, err
	}
	var body interface{}
	if in.Head != "" {
		//新建一种类型
		if t, exists := c.p.types[in.Head]; exists {
			body = reflect.New(t).Interface()
		}
	}
	err = json.Unmarshal(*in.Body, &body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (c *jsonCodec) Send(msg interface{}) error {
	var out jsonOut
	t := reflect.TypeOf(msg)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	//根据类型获取对应的head的名字
	if name, exists := c.p.names[t]; exists {
		out.Head = name
	}
	//存储消息内容，编码
	out.Body = msg
	return c.encoder.Encode(&out)
}

//关闭
func (c *jsonCodec) Close() error {
	if c.closer != nil {
		return c.closer.Close()
	}
	return nil
}
