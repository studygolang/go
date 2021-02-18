// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// 包 encoding 定义了其他包的共享接口，这些包将数据在字节级表示形式和文本表示形式相互转换。
// encoding/gob、encoding/json、encoding/xml三个包都会检查使用这些接口。
// 因此，只要实现了这些接口一次，就可以在多个包里使用。
// 标准包内建类型 time.Time和 net.IP都实现了这些接口。
// 接口是成对的，分别产生和还原编码后的数据。
package encoding

// 实现了 BinaryMarshaler 接口的类型可以将自身序列化为二进制形式。
//
// MarshalBinary 将接收到的数据编码为二进制形式并返回结果。
type BinaryMarshaler interface {
	MarshalBinary() (data []byte, err error)
}

// 实现了 BinaryUnmarshaler 接口的类型可以将二进制表示的自身反序列化。
//
// UnmarshalBinary 必须能够解析 MarshalBinary 生成的格式。
// 如果要在返回后保留数据，UnmarshalBinary 必须复制数据。
type BinaryUnmarshaler interface {
	UnmarshalBinary(data []byte) error
}

// 实现了 BinaryMarshaler 接口的类型可以将自身序列化为 utf-8 编码的文本格式。
//
// MarshalText 将接收的数据编码为 UTF-8 编码的文本并返回结果。
type TextMarshaler interface {
	MarshalText() (text []byte, err error)
}

// 实现了 TextUnmarshaler 接口的类型可以将文本格式表示的自身反序列化。
//
// UnmarshalText 必须可以解码 MarshalText 生成的文本格式数据。
// 如果要在返回后保留该文本，UnmarshalText 必须复制该文本。
type TextUnmarshaler interface {
	UnmarshalText(text []byte) error
}
