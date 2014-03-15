package uzbl

import (
	"strconv"
	"strings"
)

type VariableStore struct {
	ints    map[string]int
	strings map[string]string
	floats  map[string]float64
}

func NewVariableStore() *VariableStore {
	return &VariableStore{
		ints:    make(map[string]int),
		strings: make(map[string]string),
		floats:  make(map[string]float64),
	}
}

func (vs *VariableStore) SetInt(name string, value int) {
	vs.ints[name] = value
}

func (vs *VariableStore) SetFloat(name string, value float64) {
	vs.floats[name] = value
}

func (vs *VariableStore) SetString(name string, value string) {
	vs.strings[name] = value
}

func (vs *VariableStore) GetInt(name string, def int) int {
	if i, ok := vs.ints[name]; ok {
		return i
	}
	return def
}

func (vs *VariableStore) GetFloat(name string, def float64) float64 {
	if f, ok := vs.floats[name]; ok {
		return f
	}
	return def
}

func (vs *VariableStore) GetString(name string, def string) string {
	if s, ok := vs.strings[name]; ok {
		return s
	}
	return def
}

func (v *VariableStore) evVariableSet(ev *Event) error {
	parts := strings.SplitN(ev.Detail, " ", 3)
	name, typ, value := parts[0], parts[1], parts[2]
	if value == `''` || len(value) < 2 {
		value = ""
	} else {
		value = value[1 : len(value)-1]
	}
	switch typ {
	case "str":
		v.SetString(name, value)
	case "int":
		i, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		v.SetInt(name, i)
	case "float":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		v.SetFloat(name, f)
	default:
		return ErrUnknownType{typ, value}
	}
	return nil
}
