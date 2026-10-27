package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strings"
)

// A small NBT reader/writer, enough to add one server to Minecraft's servers.dat without losing the others.

type nbtTag struct {
	Type  byte
	Name  string
	Value any // see read for the Go type of each tag
}

type nbtList struct {
	Elem  byte
	Items []any
}

func nbtRead(r io.Reader) (*nbtTag, error) {
	var t [1]byte
	if _, err := io.ReadFull(r, t[:]); err != nil {
		return nil, err
	}
	if t[0] != 10 {
		return nil, errors.New("not a compound")
	}
	name, err := readStr(r)
	if err != nil {
		return nil, err
	}
	v, err := readPayload(r, 10, 0)
	if err != nil {
		return nil, err
	}
	return &nbtTag{Type: 10, Name: name, Value: v}, nil
}

func readStr(r io.Reader) (string, error) {
	var n uint16
	if err := binary.Read(r, binary.BigEndian, &n); err != nil {
		return "", err
	}
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	return string(b), err
}

func readPayload(r io.Reader, t byte, depth int) (any, error) {
	if depth > 64 {
		return nil, errors.New("too deep")
	}
	switch t {
	case 1:
		var v int8
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 2:
		var v int16
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 3:
		var v int32
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 4:
		var v int64
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 5:
		var v float32
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 6:
		var v float64
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 7:
		var n int32
		if err := binary.Read(r, binary.BigEndian, &n); err != nil || n < 0 {
			return nil, errors.New("bad byte array")
		}
		b := make([]byte, n)
		_, err := io.ReadFull(r, b)
		return b, err
	case 8:
		return readStr(r)
	case 9:
		var et [1]byte
		if _, err := io.ReadFull(r, et[:]); err != nil {
			return nil, err
		}
		var n int32
		if err := binary.Read(r, binary.BigEndian, &n); err != nil || n < 0 {
			return nil, errors.New("bad list")
		}
		l := &nbtList{Elem: et[0]}
		for i := int32(0); i < n; i++ {
			v, err := readPayload(r, et[0], depth+1)
			if err != nil {
				return nil, err
			}
			l.Items = append(l.Items, v)
		}
		return l, nil
	case 10:
		var tags []*nbtTag
		for {
			var tt [1]byte
			if _, err := io.ReadFull(r, tt[:]); err != nil {
				return nil, err
			}
			if tt[0] == 0 {
				return tags, nil
			}
			name, err := readStr(r)
			if err != nil {
				return nil, err
			}
			v, err := readPayload(r, tt[0], depth+1)
			if err != nil {
				return nil, err
			}
			tags = append(tags, &nbtTag{Type: tt[0], Name: name, Value: v})
		}
	case 11, 12:
		var n int32
		if err := binary.Read(r, binary.BigEndian, &n); err != nil || n < 0 {
			return nil, errors.New("bad array")
		}
		size := 4
		if t == 12 {
			size = 8
		}
		b := make([]byte, int(n)*size)
		_, err := io.ReadFull(r, b)
		return rawArray{t, n, b}, err
	}
	return nil, errors.New("unknown tag")
}

type rawArray struct {
	t    byte
	n    int32
	data []byte
}

func writeStr(w *bytes.Buffer, s string) {
	_ = binary.Write(w, binary.BigEndian, uint16(len(s)))
	w.WriteString(s)
}

func writePayload(w *bytes.Buffer, t byte, v any) {
	switch t {
	case 1, 2, 3, 4, 5, 6:
		_ = binary.Write(w, binary.BigEndian, v)
	case 7:
		b := v.([]byte)
		_ = binary.Write(w, binary.BigEndian, int32(len(b)))
		w.Write(b)
	case 8:
		writeStr(w, v.(string))
	case 9:
		l := v.(*nbtList)
		w.WriteByte(l.Elem)
		_ = binary.Write(w, binary.BigEndian, int32(len(l.Items)))
		for _, it := range l.Items {
			writePayload(w, l.Elem, it)
		}
	case 10:
		for _, tag := range v.([]*nbtTag) {
			w.WriteByte(tag.Type)
			writeStr(w, tag.Name)
			writePayload(w, tag.Type, tag.Value)
		}
		w.WriteByte(0)
	case 11, 12:
		a := v.(rawArray)
		_ = binary.Write(w, binary.BigEndian, a.n)
		w.Write(a.data)
	}
}

func nbtWrite(root *nbtTag) []byte {
	var w bytes.Buffer
	w.WriteByte(10)
	writeStr(&w, root.Name)
	writePayload(&w, 10, root.Value)
	return w.Bytes()
}

func strField(tags []*nbtTag, name string) string {
	for _, t := range tags {
		if t.Name == name && t.Type == 8 {
			return t.Value.(string)
		}
	}
	return ""
}

// addServer puts our server first in servers.dat (or updates its name) and keeps every other entry.
func addServer(path, name, address string) error {
	root := &nbtTag{Type: 10, Name: "", Value: []*nbtTag{}}
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		if r, err := nbtRead(bytes.NewReader(b)); err == nil {
			root = r
		} else {
			_ = os.WriteFile(path+".broken-installer", b, 0o644)
		}
		_ = os.WriteFile(path+".bak-installer", b, 0o644)
	}
	tags := root.Value.([]*nbtTag)
	var list *nbtList
	for _, t := range tags {
		if t.Name == "servers" && t.Type == 9 {
			list = t.Value.(*nbtList)
		}
	}
	if list == nil {
		list = &nbtList{Elem: 10}
		tags = append(tags, &nbtTag{Type: 9, Name: "servers", Value: list})
	}
	if list.Elem != 10 {
		list.Elem, list.Items = 10, nil
	}
	var keep []any
	for _, it := range list.Items {
		c := it.([]*nbtTag)
		if strings.EqualFold(strField(c, "ip"), address) {
			continue // replaced by the fresh entry below
		}
		keep = append(keep, c)
	}
	entry := []*nbtTag{
		{Type: 8, Name: "name", Value: name},
		{Type: 8, Name: "ip", Value: address},
		{Type: 1, Name: "acceptTextures", Value: int8(1)},
	}
	list.Items = append([]any{entry}, keep...)
	root.Value = tags
	return writeAtomic(path, nbtWrite(root))
}
