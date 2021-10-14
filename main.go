package main

import (
	"fmt"
	"net/http"
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
		v, found = dm.Get(injectFrom)
	} else {
		v, found = dm.Get(field.Name)
	}
	if !found {
		dm.setByName(item, field.Name, reflect.Zero(field.Type))
	} else {
		dm.setByName(item, field.Name, v)
	}
}

func (dm DependencyManager) constructFromType(aType reflect.Type) (interface{}, bool) {
	result := reflect.New(aType)
	for i := 0; i < aType.NumField(); i++ {
		dm.setField(result, aType.Field(i))
	}
	return result.Elem().Interface(), true
}

func (dm DependencyManager) returnValue(anInstance interface{}) (interface{}, bool) {
	t, v := reflect.TypeOf(anInstance), reflect.ValueOf(anInstance)

	if t.Kind() == reflect.Func {
		results := v.Call([]reflect.Value{reflect.ValueOf(dm)})
		return results[0].Interface(), results[1].Bool()
	} else if t.Kind() == reflect.Struct {
		return anInstance, true
	} else {
		return anInstance, true
	}
}

func (dm DependencyManager) Get(name string) (interface{}, bool) {
	anInstance, ok := dm.guts[name]
	if aType, hasType := dm.types[name]; !ok && hasType {
		return dm.constructFromType(aType)
	}

	return dm.returnValue(anInstance)
}

func (dm DependencyManager) MustGet(name string) interface{} {
	someAsset, ok := dm.Get(name)
	if !ok {
		panic(fmt.Sprintf("failed to find a registered value for %s", name))
	}

	return someAsset
}

func (dm DependencyManager) Inject(someType reflect.Type) interface{} {
	result, _ := dm.constructFromType(someType)
	return result
}

type StorageService struct {
	Host    string
	Port    int
	Account string `inject:"accountId"`
}

type StorageHandler struct {
	Service StorageService `inject:"storageConn"`
}

func (sh StorageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

}

func main() {
	dm := DependencyManager{
		guts:  make(map[string]interface{}),
		types: map[string]reflect.Type{},
	}

	dm.Register("storageEndpoint", StorageService{Host: "foo.bar.com", Port: 1234})
	dm.Register("accountId", "ABC123")
	dm.Register("storageConn", func(dm DependencyManager) (interface{}, bool) {
		var endpoint StorageService
		var ok bool
		if endpoint, ok = dm.MustGet("storageEndpoint").(StorageService); !ok {
			return nil, false
		}

		var accountId string
		if accountId, ok = dm.MustGet("accountId").(string); !ok {
			return nil, false
		}

		endpoint.Account = accountId
		return endpoint, ok
	})
	dm.Register("storageHandler", reflect.TypeOf(StorageHandler{}))

	se, _ := dm.Get("storageConn")
	if endpoint, ok := se.(StorageService); ok {
		fmt.Printf("Connecting to %s:%d with %s\n", endpoint.Host, endpoint.Port, endpoint.Account)
	}

	if sh, ok := dm.MustGet("storageHandler").(StorageHandler); !ok {
		fmt.Printf("Failed to set up application - exiting")
	} else {
		fmt.Printf("Connecting to %s:%d with %s\n", sh.Service.Host, sh.Service.Port, sh.Service.Account)
	}

	if sh, ok := dm.Inject(reflect.TypeOf(StorageHandler{})).(StorageHandler); ok {
		fmt.Printf("Connecting to %s:%d with %s\n", sh.Service.Host, sh.Service.Port, sh.Service.Account)
	} else {
		fmt.Printf("Failed to set up application - exiting")
	}
}
