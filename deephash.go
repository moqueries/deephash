package deephash

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"reflect"
	"sort"
)

// Traverses recursively hashing each exported value
// During deepHash, must keep track of visited, to avoid circular traversal.
// The algorithm is based on: https://github.com/imdario/mergo
func deepHash(src reflect.Value, visited map[uintptr][]reflect.Type) ([]byte, error) {
	if !src.IsValid() {
		return nil, nil
	}
	if src.CanAddr() {
		addr := src.UnsafeAddr()
		h := addr
		seen := visited[h]
		newType := src.Type()
		for _, typ := range seen {
			if typ == newType {
				return nil, nil
			}
		}
		// Remember, remember...
		visited[h] = append(seen, newType)
	}

	hash := fnv.New64a()

	// deal with pointers/interfaces
	for src.Kind() == reflect.Ptr || src.Kind() == reflect.Interface {
		src = src.Elem()
	}

	switch src.Kind() {
	case reflect.Struct:
		for i, n := 0, src.NumField(); i < n; i++ {
			b, err := deepHash(src.Field(i), visited)
			if err != nil {
				return nil, err
			}
			if b != nil {
				_, err := hash.Write(b)
				if err != nil {
					return nil, err
				}
			}
		}
	case reflect.Map:
		sortedHashedKeys := make([]string, len(src.MapKeys()))
		indexedByHash := make(map[string]reflect.Value)

		for i, key := range src.MapKeys() {
			h, err := deepHash(key, visited)
			if err != nil {
				return nil, err
			}
			kh := fmt.Sprintf("%x", h)
			sortedHashedKeys[i] = kh
			indexedByHash[kh] = src.MapIndex(key)
		}
		sort.Strings(sortedHashedKeys)

		// hash each value, in order
		for _, kh := range sortedHashedKeys {
			_, err := hash.Write([]byte(kh))
			if err != nil {
				return nil, err
			}
			var h []byte
			h, err = deepHash(indexedByHash[kh], visited)
			if err != nil {
				return nil, err
			}
			_, err = hash.Write(h)
			if err != nil {
				return nil, err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < src.Len(); i++ {
			h, err := deepHash(src.Index(i), visited)
			if err != nil {
				return nil, err
			}
			_, err = hash.Write(h)
			if err != nil {
				return nil, err
			}
		}
	case reflect.String:
		_, err := hash.Write([]byte(src.String()))
		if err != nil {
			return nil, err
		}
	case reflect.Bool:
		if src.Bool() {
			_, err := hash.Write([]byte("1"))
			if err != nil {
				return nil, err
			}
		} else {
			_, err := hash.Write([]byte("0"))
			if err != nil {
				return nil, err
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		err := binary.Write(hash, binary.BigEndian, src.Int())
		if err != nil {
			return nil, err
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		err := binary.Write(hash, binary.BigEndian, src.Uint())
		if err != nil {
			return nil, err
		}
	case reflect.Float32, reflect.Float64:
		err := binary.Write(hash, binary.BigEndian, src.Float())
		if err != nil {
			return nil, err
		}
	}

	return hash.Sum(nil), nil
}

// Hash returns a fnv64a hash of src, hashing recursively any exported
// properties, including slices and maps/
func Hash(src interface{}) []byte {
	vSrc := reflect.ValueOf(src)
	h, err := deepHash(vSrc, make(map[uintptr][]reflect.Type))
	if err != nil {
		panic(err)
	}
	return h
}
