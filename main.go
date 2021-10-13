package main

import (
	"fmt"
	"reflect"
)

type DependencyManager struct {
	guts  map[string]interface{}
	types map[string]reflect.Type
}

func (dm *DependencyManager) Register(name string, item interface{}) {
	if _, ok := item.(reflect.Type); ok {
		dm.types[name] = item.(reflect.Type)
	} else {
		dm.guts[name] = item
	}
}

func (dm DependencyManager) setByName(item reflect.Value, fieldName string, val interface{}) {
	field := item.Elem().FieldByName(fieldName)

	switch {
	case reflect.TypeOf(val).Kind() == field.Kind():
		field.Set(reflect.ValueOf(val))
	case reflect.TypeOf(val).Kind() == reflect.Ptr:
		field.Set(reflect.ValueOf(val).Elem())
	case field.Kind() == reflect.Ptr:
		field.Set(reflect.ValueOf(&val))
	}
}

func (dm DependencyManager) setField(item reflect.Value, field reflect.StructField) {
	injectFrom, ok := field.Tag.Lookup("inject")
	var v interface{}
	var found bool
	if ok {
		v, found = dm.GetInstance(injectFrom)
	} else {
		v, found = dm.GetInstance(field.Name)
	}
	if !found {
		dm.setByName(item, field.Name, reflect.Zero(field.Type))
	} else {
		dm.setByName(item, field.Name, v)
	}
}

func (dm DependencyManager) GetInstance(name string) (interface{}, bool) {
	anInstance, ok := dm.guts[name]
	if !ok {
		if someType, ok := dm.types[name]; !ok {
			return nil, false
		} else {
			result := reflect.New(someType)
			for i := 0; i < someType.NumField(); i++ {
				dm.setField(result, someType.Field(i))
			}
			return result.Interface(), true
		}
	}

	t, v := reflect.TypeOf(anInstance), reflect.ValueOf(anInstance)

	if t.Kind() == reflect.Func {
		results := v.Call([]reflect.Value{reflect.ValueOf(dm)})
		return results[0].Interface(), ok
	} else if t.Kind() == reflect.Struct {
		// Todo: If the inject tag is present, and the value is zero, inject it.
		return anInstance, ok
	} else {
		return anInstance, ok
	}
}

type Person struct {
	FirstName, LastName string
}

func (p Person) String() string {
	return fmt.Sprintf("%s, %s", p.LastName, p.FirstName)
}

func main() {
	dm := DependencyManager{
		guts:  make(map[string]interface{}),
		types: map[string]reflect.Type{},
	}

	t := reflect.TypeOf(Person{})

	//dm.Register("FirstName", "Joe")
	//dm.Register("LastName", "User")
	dm.Register("Person", t)
	dm.Register("Ed", Person{
		FirstName: "Ed",
		LastName:  "Smith",
	})

	val, found := dm.GetInstance("Person")
	if !found {
		fmt.Println("Did not find the person")
	}
	fmt.Printf("%v\n", val)

	val, found = dm.GetInstance("Ed")
	if !found {
		fmt.Println("Ed went missing")
	}

	if ed, ok := val.(Person); !ok {
		fmt.Println("What we got back wasn't Ed")
	} else {
		fmt.Printf("%v\n", ed)
	}
}
