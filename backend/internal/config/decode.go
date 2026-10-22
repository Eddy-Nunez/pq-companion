package config

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// DecodeOver decodes a JSON config body over c. Fields the body omits keep
// their current value (so an older or partial client can't wipe a setting),
// but any map the body does send replaces the current map outright.
//
// encoding/json merges into an existing map and never deletes keys, so
// decoding over c alone would make removing a map entry impossible — e.g.
// turning off a character's hidden-items override, or deleting a raid threat
// player mod. Clients always send a map's full contents, so replacing it is
// the intended meaning.
func DecodeOver(c *Config, data []byte) error {
	// Decode into a zero Config as well: its maps start nil, so a non-nil map
	// afterwards means the body sent that field.
	var sent Config
	if err := json.Unmarshal(data, &sent); err != nil {
		return fmt.Errorf("decode config into zero value: %w", err)
	}
	if err := json.Unmarshal(data, c); err != nil {
		return fmt.Errorf("decode config over current: %w", err)
	}
	replaceSentMaps(reflect.ValueOf(c).Elem(), reflect.ValueOf(&sent).Elem())
	return nil
}

// replaceSentMaps copies every non-nil map in sent onto the matching field of
// dst, walking nested structs, pointers to structs, and slice elements.
func replaceSentMaps(dst, sent reflect.Value) {
	switch dst.Kind() {
	case reflect.Map:
		if !sent.IsNil() && dst.CanSet() {
			dst.Set(sent)
		}
	case reflect.Pointer:
		if !dst.IsNil() && !sent.IsNil() {
			replaceSentMaps(dst.Elem(), sent.Elem())
		}
	case reflect.Struct:
		for i := 0; i < dst.NumField(); i++ {
			if dst.Type().Field(i).IsExported() {
				replaceSentMaps(dst.Field(i), sent.Field(i))
			}
		}
	case reflect.Slice:
		// A sent slice is decoded over the current elements index by index,
		// so element i of both lines up.
		if dst.Len() == sent.Len() {
			for i := 0; i < dst.Len(); i++ {
				replaceSentMaps(dst.Index(i), sent.Index(i))
			}
		}
	}
}
