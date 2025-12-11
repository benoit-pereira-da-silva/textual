// Copyright 2026 Benoit Pereira da Silva
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package textual

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// UTF8String is used for code expressivity.
// We manipulate many encodings, but our internal string representation is always in utf8
type UTF8String = string

// EncodingID is an enum-like type for supported encodings.
type EncodingID int

const (
	UTF8 EncodingID = iota
	UTF16LE
	UTF16BE
	UTF16LEBOM
	UTF16BEBOM

	ISO8859_1
	ISO8859_2
	ISO8859_3
	ISO8859_4
	ISO8859_5
	ISO8859_6
	ISO8859_7
	ISO8859_8
	ISO8859_9
	ISO8859_10
	ISO8859_13
	ISO8859_14
	ISO8859_15
	ISO8859_16

	KOI8R
	KOI8U

	Windows874
	Windows1250
	Windows1251
	Windows1252
	Windows1253
	Windows1254
	Windows1255
	Windows1256
	Windows1257
	Windows1258

	MacRoman
	MacCyrillic

	ShiftJIS
	EUCJP
	ISO2022JP

	GBK
	HZGB2312
	GB18030

	Big5

	EUCKR
)

// EncodingName returns a canonical string name.
func (e EncodingID) EncodingName() string {
	switch e {
	case UTF8:
		return "UTF-8"
	case UTF16LE:
		return "UTF-16LE"
	case UTF16BE:
		return "UTF-16BE"
	case UTF16LEBOM:
		return "UTF-16LE-BOM"
	case UTF16BEBOM:
		return "UTF-16BE-BOM"

	case ISO8859_1:
		return "ISO-8859-1"
	case ISO8859_2:
		return "ISO-8859-2"
	case ISO8859_3:
		return "ISO-8859-3"
	case ISO8859_4:
		return "ISO-8859-4"
	case ISO8859_5:
		return "ISO-8859-5"
	case ISO8859_6:
		return "ISO-8859-6"
	case ISO8859_7:
		return "ISO-8859-7"
	case ISO8859_8:
		return "ISO-8859-8"
	case ISO8859_9:
		return "ISO-8859-9"
	case ISO8859_10:
		return "ISO-8859-10"
	case ISO8859_13:
		return "ISO-8859-13"
	case ISO8859_14:
		return "ISO-8859-14"
	case ISO8859_15:
		return "ISO-8859-15"
	case ISO8859_16:
		return "ISO-8859-16"

	case KOI8R:
		return "KOI8-R"
	case KOI8U:
		return "KOI8-U"

	case Windows874:
		return "Windows-874"
	case Windows1250:
		return "Windows-1250"
	case Windows1251:
		return "Windows-1251"
	case Windows1252:
		return "Windows-1252"
	case Windows1253:
		return "Windows-1253"
	case Windows1254:
		return "Windows-1254"
	case Windows1255:
		return "Windows-1255"
	case Windows1256:
		return "Windows-1256"
	case Windows1257:
		return "Windows-1257"
	case Windows1258:
		return "Windows-1258"

	case MacRoman:
		return "MacRoman"
	case MacCyrillic:
		return "MacCyrillic"
	case ShiftJIS:
		return "ShiftJIS"
	case EUCJP:
		return "EUC-JP"
	case ISO2022JP:
		return "ISO-2022-JP"

	case GBK:
		return "GBK"
	case HZGB2312:
		return "HZ-GB2312"
	case GB18030:
		return "GB18030"

	case Big5:
		return "Big5"

	case EUCKR:
		return "EUC-KR"
	}
	return "Unknown"
}

// nameToEncoding maps lower-case names to enum.
var nameToEncoding = map[string]EncodingID{
	"utf-8":        UTF8,
	"utf8":         UTF8,
	"utf-16le":     UTF16LE,
	"utf-16be":     UTF16BE,
	"utf-16le-bom": UTF16LEBOM,
	"utf-16be-bom": UTF16BEBOM,

	"iso-8859-1":  ISO8859_1,
	"iso-8859-2":  ISO8859_2,
	"iso-8859-3":  ISO8859_3,
	"iso-8859-4":  ISO8859_4,
	"iso-8859-5":  ISO8859_5,
	"iso-8859-6":  ISO8859_6,
	"iso-8859-7":  ISO8859_7,
	"iso-8859-8":  ISO8859_8,
	"iso-8859-9":  ISO8859_9,
	"iso-8859-10": ISO8859_10,
	"iso-8859-13": ISO8859_13,
	"iso-8859-14": ISO8859_14,
	"iso-8859-15": ISO8859_15,
	"iso-8859-16": ISO8859_16,

	"koi8-r": KOI8R,
	"koi8-u": KOI8U,

	"windows-874":  Windows874,
	"windows-1250": Windows1250,
	"windows-1251": Windows1251,
	"windows-1252": Windows1252,
	"windows-1253": Windows1253,
	"windows-1254": Windows1254,
	"windows-1255": Windows1255,
	"windows-1256": Windows1256,
	"windows-1257": Windows1257,
	"windows-1258": Windows1258,

	"macroman":    MacRoman,
	"maccyrillic": MacCyrillic,

	"shiftjis":    ShiftJIS,
	"shift-jis":   ShiftJIS,
	"euc-jp":      EUCJP,
	"iso-2022-jp": ISO2022JP,

	"gbk":       GBK,
	"hz-gb2312": HZGB2312,
	"gb18030":   GB18030,

	"big5": Big5,

	"euc-kr": EUCKR,
}

// ParseEncoding returns the EncodingID for a given name (case-insensitive).
func ParseEncoding(name string) (EncodingID, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if enc, ok := nameToEncoding[key]; ok {
		return enc, nil
	}
	return 0, fmt.Errorf("unknown encoding: %s", name)
}

// GetEncoding returns the encoding.Encoding instance.
func GetEncoding(e EncodingID) (encoding.Encoding, error) {
	switch e {
	case UTF8:
		return unicode.UTF8, nil
	case UTF16LE:
		return unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM), nil
	case UTF16BE:
		return unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM), nil
	case UTF16LEBOM:
		return unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM), nil
	case UTF16BEBOM:
		return unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM), nil

	case ISO8859_1:
		return charmap.ISO8859_1, nil
	case ISO8859_2:
		return charmap.ISO8859_2, nil
	case ISO8859_3:
		return charmap.ISO8859_3, nil
	case ISO8859_4:
		return charmap.ISO8859_4, nil
	case ISO8859_5:
		return charmap.ISO8859_5, nil
	case ISO8859_6:
		return charmap.ISO8859_6, nil
	case ISO8859_7:
		return charmap.ISO8859_7, nil
	case ISO8859_8:
		return charmap.ISO8859_8, nil
	case ISO8859_9:
		return charmap.ISO8859_9, nil
	case ISO8859_10:
		return charmap.ISO8859_10, nil
	case ISO8859_13:
		return charmap.ISO8859_13, nil
	case ISO8859_14:
		return charmap.ISO8859_14, nil
	case ISO8859_15:
		return charmap.ISO8859_15, nil
	case ISO8859_16:
		return charmap.ISO8859_16, nil

	case KOI8R:
		return charmap.KOI8R, nil
	case KOI8U:
		return charmap.KOI8U, nil

	case Windows874:
		return charmap.Windows874, nil
	case Windows1250:
		return charmap.Windows1250, nil
	case Windows1251:
		return charmap.Windows1251, nil
	case Windows1252:
		return charmap.Windows1252, nil
	case Windows1253:
		return charmap.Windows1253, nil
	case Windows1254:
		return charmap.Windows1254, nil
	case Windows1255:
		return charmap.Windows1255, nil
	case Windows1256:
		return charmap.Windows1256, nil
	case Windows1257:
		return charmap.Windows1257, nil
	case Windows1258:
		return charmap.Windows1258, nil
	case MacRoman:
		return charmap.Macintosh, nil
	case MacCyrillic:
		return charmap.MacintoshCyrillic, nil
	case ShiftJIS:
		return japanese.ShiftJIS, nil
	case EUCJP:
		return japanese.EUCJP, nil
	case ISO2022JP:
		return japanese.ISO2022JP, nil

	case GBK:
		return simplifiedchinese.GBK, nil
	case HZGB2312:
		return simplifiedchinese.HZGB2312, nil
	case GB18030:
		return simplifiedchinese.GB18030, nil

	case Big5:
		return traditionalchinese.Big5, nil

	case EUCKR:
		return korean.EUCKR, nil
	}

	return nil, errors.New("unsupported encoding id")
}

// NewUTF8Reader wraps r in a streaming decoder that converts from the
// specified source encoding into UTF‑8 on the fly.
//
// The returned reader can be plugged directly into components such as
// bufio.Scanner or IOReaderProcessor, avoiding the need to load the whole
// content into memory first.
//
// Callers remain responsible for closing r if it also implements io.Closer.
func NewUTF8Reader(r io.Reader, src EncodingID) (io.Reader, error) {
	enc, err := GetEncoding(src)
	if err != nil {
		return nil, err
	}
	return transform.NewReader(r, enc.NewDecoder()), nil
}

// ToUTF8 converts bytes (in any encoding) to UTF‑8.
//
// This is a convenience wrapper around ReaderToUTF8 for in‑memory data.
func ToUTF8(input []byte, src EncodingID) (UTF8String, error) {
	r := bytes.NewReader(input)
	return ReaderToUTF8(r, src)
}

// ReaderToUTF8 reads all data from r, decodes it from the specified source
// encoding, and returns it as a UTF‑8 string.
func ReaderToUTF8(r io.Reader, src EncodingID) (UTF8String, error) {
	reader, err := NewUTF8Reader(r, src)
	if err != nil {
		return "", err
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return UTF8String(out), nil
}

// FromUTF8 encodes a UTF-8 string into a target encoding.
func FromUTF8(input UTF8String, dest EncodingID) ([]byte, error) {
	var buf bytes.Buffer
	// Reuse the streaming variant, writing into our buffer.
	if err := FromUTF8ToWriter(input, dest, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// FromUTF8ToWriter encodes a UTF-8 into a target encoding.
func FromUTF8ToWriter(input UTF8String, dest EncodingID, writer io.Writer) error {
	enc, err := GetEncoding(dest)
	if err != nil {
		return err
	}
	reader := transform.NewReader(strings.NewReader(string(input)), enc.NewEncoder())
	// Stream the transformed data into the provided writer.
	_, err = io.Copy(writer, reader)
	if err != nil {
		return err
	}
	return nil
}
