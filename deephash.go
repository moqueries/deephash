package deephash

import (
	"bytes"
	"encoding/binary"
	"hash/fnv"
	"io"
	"reflect"
	"sort"
	"strconv"
)

const notEq = " is not equal"

// Hash returns a fnv64a hash of src, hashing recursively any exported
// properties, including slices and maps/
func Hash(src interface{}) uint64 {
	vSrc := reflect.ValueOf(src)
	h := fnv.New64a()
	err := deepHash(vSrc, "", noopFieldWriter{h}, make(map[uintptr][]reflect.Type))
	if err != nil {
		panic(err)
	}
	return h.Sum64()
}

// FastHash has a very minor performance advantage over Hash
// func FastHash(src interface{}) uint64 {
// 	vSrc := reflect.ValueOf(src)
// 	h := fnv.New64a()
// 	err := fastDeepHash(vSrc, h, make(map[uintptr][]reflect.Type))
// 	if err != nil {
// 		panic(err)
// 	}
// 	return h.Sum64()
// }

// Diff returns a list of differences between lSrc and rSrc
func Diff(field string, lSrc, rSrc interface{}) []string {
	if field == "" {
		field = "value"
	}

	cw := compareWriter{
		writes: make(map[string][]byte),
	}
	vSrc := reflect.ValueOf(lSrc)
	err := deepHash(vSrc, field, &cw, make(map[uintptr][]reflect.Type))
	if err != nil {
		panic(err)
	}

	cw.comparing = true
	vSrc = reflect.ValueOf(rSrc)
	err = deepHash(vSrc, field, &cw, make(map[uintptr][]reflect.Type))
	if err != nil {
		panic(err)
	}

	for k := range cw.writes {
		cw.diffs = append(cw.diffs, k+notEq)
	}

	return cw.diffs
}

// fieldWriter writes individual fields to a writer
type fieldWriter interface {
	Write(f string, p []byte) error
}

// noopFieldWriter writes fields to a writer but ignores the field name
type noopFieldWriter struct {
	io.Writer
}

func (w noopFieldWriter) Write(_ string, p []byte) error {
	_, err := w.Writer.Write(p)
	return err
}

// captureWriter captures the []byte when written to using the io.Writer
// interface. It panics if Write is called twice.
type captureWriter struct {
	c []byte
}

func (w *captureWriter) Write(p []byte) (int, error) {
	if w.c != nil {
		panic("captureWriter cannot be written to more than once")
	}

	w.c = p

	return len(p), nil
}

// compareWriter stores binary representations of fields to be compared when
// comparing is false. When comparing is true and subsequent calls are made,
// differing fields are recorded to diffs.
type compareWriter struct {
	writes    map[string][]byte
	diffs     []string
	comparing bool
}

func (w *compareWriter) Write(f string, p []byte) error {
	if !w.comparing {
		w.writes[f] = p
		return nil
	}

	prevP, ok := w.writes[f]
	if !ok || !bytes.Equal(p, prevP) {
		if f == "" {
			f = "value"
		}

		w.diffs = append(w.diffs, f+notEq)
	}
	delete(w.writes, f)

	return nil
}

type mapElement struct {
	kh   uint64
	k, v reflect.Value
}

// Traverses recursively hashing each exported value
// During deepHash, must keep track of visited, to avoid circular traversal.
// The algorithm is based on: https://github.com/imdario/mergo
func deepHash(src reflect.Value, field string, h fieldWriter, visited map[uintptr][]reflect.Type) error {
	if !src.IsValid() {
		return nil
	}
	if src.CanAddr() {
		addr := src.UnsafeAddr()
		h := addr
		seen, previouslySeen := visited[h]
		newType := src.Type()
		for _, typ := range seen {
			if typ == newType {
				return nil
			}
		}
		// Remember, remember...
		visited[h] = append(seen, newType)
		defer func() {
			// If we get here, we've either added a new entry in visited or
			// a new type to the end of a slice in visited
			if previouslySeen {
				// If we just added a type to the end, remove it when
				// returning from this level of recursion
				prev := visited[h]
				visited[h] = prev[0 : len(prev)-1]
			} else {
				// If this is the first time we've seen this memory address,
				// pop it off when returning from this level of recursion
				delete(visited, h)
			}
		}()
	}

	// deal with pointers/interfaces
	for src.Kind() == reflect.Ptr || src.Kind() == reflect.Interface {
		src = src.Elem()
	}

	var cw captureWriter
	switch src.Kind() {
	case reflect.Struct:
		for i, n := 0, src.NumField(); i < n; i++ {
			var name string
			if field != "" {
				f := src.Type().Field(i)
				name = appendName(field, f.Name, defaultType)
			}
			err := deepHash(src.Field(i), name, h, visited)
			if err != nil {
				return err
			}
		}
	case reflect.Map:
		elements := make([]mapElement, len(src.MapKeys()))

		for i, key := range src.MapKeys() {
			subH := fnv.New64a()
			kh := noopFieldWriter{subH}
			err := deepHash(key, "", kh, visited)
			if err != nil {
				return err
			}
			elements[i] = mapElement{
				kh: subH.Sum64(),
				k:  key,
				v:  src.MapIndex(key),
			}
		}
		sort.Slice(elements, func(i, j int) bool {
			return elements[i].kh < elements[j].kh
		})

		// hash each value, in order
		for _, el := range elements {
			cw := captureWriter{}
			err := binary.Write(&cw, binary.BigEndian, el.kh)
			if err != nil {
				return err
			}
			err = h.Write(appendName(field, el.k.String(), mapKeyType), cw.c)
			if err != nil {
				return err
			}

			err = deepHash(el.v, appendName(field, el.k.String(), indexedType), h, visited)
			if err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < src.Len(); i++ {
			err := deepHash(src.Index(i), appendName(field, strconv.Itoa(i), indexedType), h, visited)
			if err != nil {
				return err
			}
		}
	case reflect.String:
		err := h.Write(field, []byte(src.String()))
		if err != nil {
			return err
		}
	case reflect.Bool:
		if src.Bool() {
			err := h.Write(field, []byte("1"))
			if err != nil {
				return err
			}
		} else {
			err := h.Write(field, []byte("0"))
			if err != nil {
				return err
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		err := binary.Write(&cw, binary.BigEndian, src.Int())
		if err != nil {
			return err
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		err := binary.Write(&cw, binary.BigEndian, src.Uint())
		if err != nil {
			return err
		}
	case reflect.Float32, reflect.Float64:
		err := binary.Write(&cw, binary.BigEndian, src.Float())
		if err != nil {
			return err
		}
	}

	if cw.c == nil {
		return nil
	}

	err := h.Write(field, cw.c)
	if err != nil {
		return err
	}

	return nil
}

type namedType int

const (
	defaultType = namedType(iota)
	mapKeyType
	indexedType
)

func appendName(base, field string, nt namedType) string {
	if base == "" {
		return ""
	}

	prefix := "["
	suffix := "]"
	switch nt {
	case defaultType:
		prefix = "."
		suffix = ""
	case mapKeyType:
		suffix = "-key" + suffix
	case indexedType:
	default:
		panic(nt)
	}

	return base + prefix + field + suffix
}

// fastDeepHash has a very minor performance advantage over deepHash
// func fastDeepHash(src reflect.Value, h io.Writer, visited map[uintptr][]reflect.Type) error {
// 	if !src.IsValid() {
// 		return nil
// 	}
// 	if src.CanAddr() {
// 		addr := src.UnsafeAddr()
// 		h := addr
// 		seen, previouslySeen := visited[h]
// 		newType := src.Type()
// 		for _, typ := range seen {
// 			if typ == newType {
// 				return nil
// 			}
// 		}
// 		// Remember, remember...
// 		visited[h] = append(seen, newType)
// 		defer func() {
// 			// If we get here, we've either added a new entry in visited or
// 			// a new type to the end of a slice in visited
// 			if previouslySeen {
// 				// If we just added a type to the end, remove it when
// 				// returning from this level of recursion
// 				prev := visited[h]
// 				visited[h] = prev[0 : len(prev)-1]
// 			} else {
// 				// If this is the first time we've seen this memory address,
// 				// pop it off when returning from this level of recursion
// 				delete(visited, h)
// 			}
// 		}()
// 	}
//
// 	// deal with pointers/interfaces
// 	for src.Kind() == reflect.Ptr || src.Kind() == reflect.Interface {
// 		src = src.Elem()
// 	}
//
// 	var cw captureWriter
// 	switch src.Kind() {
// 	case reflect.Struct:
// 		for i, n := 0, src.NumField(); i < n; i++ {
// 			err := fastDeepHash(src.Field(i), h, visited)
// 			if err != nil {
// 				return err
// 			}
// 		}
// 	case reflect.Map:
// 		elements := make([]mapElement, len(src.MapKeys()))
//
// 		for i, key := range src.MapKeys() {
// 			subH := fnv.New64a()
// 			err := fastDeepHash(key, subH, visited)
// 			if err != nil {
// 				return err
// 			}
// 			elements[i] = mapElement{
// 				kh: subH.Sum64(),
// 				k:  key,
// 				v:  src.MapIndex(key),
// 			}
// 		}
// 		sort.Slice(elements, func(i, j int) bool {
// 			return elements[i].kh < elements[j].kh
// 		})
//
// 		// hash each value, in order
// 		for _, el := range elements {
// 			err := binary.Write(h, binary.BigEndian, el.kh)
// 			if err != nil {
// 				return err
// 			}
//
// 			err = fastDeepHash(el.v, h, visited)
// 			if err != nil {
// 				return err
// 			}
// 		}
// 	case reflect.Slice, reflect.Array:
// 		for i := 0; i < src.Len(); i++ {
// 			err := fastDeepHash(src.Index(i), h, visited)
// 			if err != nil {
// 				return err
// 			}
// 		}
// 	case reflect.String:
// 		_, err := h.Write([]byte(src.String()))
// 		if err != nil {
// 			return err
// 		}
// 	case reflect.Bool:
// 		if src.Bool() {
// 			_, err := h.Write([]byte("1"))
// 			if err != nil {
// 				return err
// 			}
// 		} else {
// 			_, err := h.Write([]byte("0"))
// 			if err != nil {
// 				return err
// 			}
// 		}
// 	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
// 		err := binary.Write(&cw, binary.BigEndian, src.Int())
// 		if err != nil {
// 			return err
// 		}
// 	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
// 		err := binary.Write(&cw, binary.BigEndian, src.Uint())
// 		if err != nil {
// 			return err
// 		}
// 	case reflect.Float32, reflect.Float64:
// 		err := binary.Write(&cw, binary.BigEndian, src.Float())
// 		if err != nil {
// 			return err
// 		}
// 	}
//
// 	if cw.c == nil {
// 		return nil
// 	}
//
// 	return nil
// }
