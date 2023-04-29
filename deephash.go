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
func deepHash(src reflect.Value, visited map[uintptr][]reflect.Type) []byte {
	if !src.IsValid() {
		return nil
	}
	if src.CanAddr() {
		addr := src.UnsafeAddr()
		h := addr
		seen := visited[h]
		newType := src.Type()
		for _, typ := range seen {
			if typ == newType {
				return nil
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
			if b := deepHash(src.Field(i), visited); b != nil {
				hash.Write(b)
			}
		}
	case reflect.Map:
		sortedHashedKeys := make([]string, len(src.MapKeys()))
		indexedByHash := make(map[string]reflect.Value)

		for i, key := range src.MapKeys() {
			kh := fmt.Sprintf("%x", deepHash(key, visited))
			sortedHashedKeys[i] = kh
			indexedByHash[kh] = src.MapIndex(key)
		}
		sort.Strings(sortedHashedKeys)

		// hash each value, in order
		for _, kh := range sortedHashedKeys {
			hash.Write([]byte(kh))
			hash.Write(deepHash(indexedByHash[kh], visited))
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < src.Len(); i++ {
			hash.Write(deepHash(src.Index(i), visited))
		}
	case reflect.String:
		hash.Write([]byte(src.String()))
	case reflect.Bool:
		if src.Bool() == true {
			hash.Write([]byte("1"))
		} else {
			hash.Write([]byte("0"))
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		binary.Write(hash, binary.BigEndian, src.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		binary.Write(hash, binary.BigEndian, src.Uint())
	case reflect.Float32, reflect.Float64:
		binary.Write(hash, binary.BigEndian, src.Float())

	}

	return hash.Sum(nil)
}

// Hash returns an fnv64a hash of src, hashing recursively any exported
// properties, including slices and maps/
func Hash(src interface{}) []byte {
	vSrc := reflect.ValueOf(src)
	return deepHash(vSrc, make(map[uintptr][]reflect.Type))
}
