package deephash

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"reflect"
	"sort"
)

// During deepMerge, must keep track of checks that are
// in progress.  The comparison algorithm assumes that all
// checks in progress are true when it reencounters them.
// Visited are stored in a map indexed by 17 * a1 + a2;
type visit struct {
	ptr  uintptr
	typ  reflect.Type
	next *visit
}

// Traverses recursively hashing each exported value
func deepHash(src reflect.Value, visited map[uintptr]*visit, depth int) []byte {
	if !src.IsValid() {
		return nil
	}
	if src.CanAddr() {
		addr := src.UnsafeAddr()
		h := 17 * addr
		seen := visited[h]
		typ := src.Type()
		for p := seen; p != nil; p = p.next {
			if p.ptr == addr && p.typ == typ {
				return nil
			}
		}
		// Remember, remember...
		visited[h] = &visit{addr, typ, seen}
	}

	hash := fnv.New64a()

	switch src.Kind() {
	case reflect.Struct:
		for i, n := 0, src.NumField(); i < n; i++ {
			if b := deepHash(src.Field(i), visited, depth+1); b != nil {
				hash.Write(b)
			}
		}
	case reflect.Map:
		sortedHashedKeys := make([]string, len(src.MapKeys()))
		indexedByHash := make(map[string]reflect.Value)

		for i, key := range src.MapKeys() {
			kh := fmt.Sprintf("%x", deepHash(key, visited, depth+1))
			sortedHashedKeys[i] = kh
			indexedByHash[kh] = src.MapIndex(key)
		}
		sort.Strings(sortedHashedKeys)

		// hash each value, in order
		for _, kh := range sortedHashedKeys {
			hash.Write([]byte(kh))
			hash.Write(deepHash(indexedByHash[kh], visited, depth+1))
		}
	case reflect.Ptr, reflect.Interface:
		if b := deepHash(src.Elem(), visited, depth+1); b != nil {
			hash.Write(b)
		}
	case reflect.String:
		hash.Write([]byte(src.String()))
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
	return deepHash(vSrc, make(map[uintptr]*visit), 0)
}
