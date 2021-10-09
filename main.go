package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v4/pgxpool"
	"log"
	"os"
	"reflect"
	"time"
)

type DependencyManager struct {
	guts map[string]interface{}
}

func (dm *DependencyManager) RegisterFactory(name string, factory interface{}) {
	dm.guts[name] = factory
}

func (dm *DependencyManager) Register(name string, item interface{}) {
	dm.guts[name] = item
}

func (dm DependencyManager) MakeInstance(name string, params ...interface{}) interface{} {
	someFactory := dm.guts[name]

	t, v := reflect.TypeOf(someFactory), reflect.ValueOf(someFactory)
	if t.Kind() != reflect.Func {
		panic("you failed to pass a function!")
	}

	callArray := make([]reflect.Value, len(params))
	for i, v := range params {
		callArray[i] = reflect.ValueOf(v)
	}

	result := v.Call(callArray)[0]
	fmt.Printf("Type of result: %v\n", result.Type())

	return result.Interface()
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
	if ok {
		v = dm.GetInstance(injectFrom)
	} else {
		v = dm.GetInstance(field.Name)
	}
	dm.setByName(item, field.Name, v)
}

func (dm DependencyManager) GetInstance(name string) interface{} {
	anInstance, ok := dm.guts[name]
	if !ok {
		return nil
	}

	t, v := reflect.TypeOf(anInstance), reflect.ValueOf(anInstance)

	if t.Kind() == reflect.Func {
		results := v.Call([]reflect.Value{reflect.ValueOf(dm)})
		return results[0].Interface()
	} else if t.Kind() == reflect.Struct {
		result := reflect.New(t)
		for i := 0; i < t.NumField(); i++ {
			dm.setField(result, t.Field(i))
		}
		return result.Interface()
	} else {
		return anInstance
	}
}

func (dm DependencyManager) GetPGXPool(name string) (*pgxpool.Pool, error) {
	i := dm.GetInstance(name)

	if myPool, ok := i.(*pgxpool.Pool); !ok {
		return nil, fmt.Errorf("failed to find pool %s", name)
	} else {
		return myPool, nil
	}
}

type MyService struct {
	SomeValue  int    `inject:"foo"`
	OtherValue string `inject:"bar"`
}

type Parent struct {
	Child Child
}

type Child struct {
	SomeValue int `inject:"foo"`
}

type Archana struct {
	SomeValue string
}

type Parent2 struct {
	Child *Child
}

type Parent3 struct {
	Child     Child
	SomeValue string `inject:"bar"`
}

func main() {
	dm := DependencyManager{
		guts: make(map[string]interface{}),
	}

	DBURI := os.Getenv("DB_URI")
	if DBURI == "" {
		log.Fatal("Failed to get db uri - bailing")
	}

	pool, err := pgxpool.Connect(context.Background(), DBURI)
	if err != nil {
		log.Fatalf("Failed to get database connection: %v", err)
	}

	dm.Register("DBURI", DBURI)
	dm.Register("pool", pool)
	dm.Register("foo", 1)
	dm.Register("bar", "baz")
	dm.Register("myService", MyService{})
	dm.Register("Child", Child{})
	dm.Register("Parent", Parent{})
	dm.Register("Parent2", Parent2{})
	dm.Register("Parent3", Parent3{})

	connectionFactory := func(dm DependencyManager) interface{} {
		var pool *pgxpool.Pool
		var ok bool

		if pool, ok = dm.GetInstance("pool").(*pgxpool.Pool); !ok {
			return nil
		}

		conn, err := pool.Acquire(context.Background())
		if err != nil {
			log.Printf("Failed to get new connection: %v", err)
			return nil
		}

		return conn
	}

	dm.Register("connection", connectionFactory)

	if v, ok := dm.GetInstance("foo").(int); ok {
		log.Printf("Got foo = %d\n", v)
	}

	if ms, ok := dm.GetInstance("myService").(*MyService); !ok {
		log.Println("Failed to get my service")
	} else {
		log.Printf("Got %d and %s", ms.SomeValue, ms.OtherValue)
	}

	if c, ok := dm.GetInstance("connection").(*pgxpool.Conn); ok {
		defer c.Release()
		row := c.QueryRow(context.Background(), "SELECT now()")
		var when time.Time
		if err := row.Scan(&when); err != nil {
			log.Printf("Failed to get time: %v", err)
		} else {
			log.Printf("Time = %v", when)
		}
	}

	if ms, ok := dm.GetInstance("Parent").(*Parent); !ok {
		log.Printf("Failed to get parent")
	} else {
		log.Printf("Got parent with child: %v", ms.Child.SomeValue)
	}

	if ms, ok := dm.GetInstance("Parent2").(*Parent2); !ok {
		log.Printf("Failed to get parent")
	} else {
		log.Printf("Got parent with child: %v", ms.Child.SomeValue)
	}

	if ms, ok := dm.GetInstance("Parent3").(*Parent3); !ok {
		log.Printf("Failed to get parent3")
	} else {
		log.Printf("Got parent with child: %v", ms.Child.SomeValue)
		log.Printf("Got value: %v", ms.SomeValue)
	}
}

func getPGXPoolExample(dm DependencyManager) {
	myPool, err := dm.GetPGXPool("pool")
	if err != nil {
		log.Printf("Did not find pool: %v", err)
		return
	}

	rows, err := myPool.Query(context.Background(), "SELECT 1;")
	if err != nil {
		log.Printf("Error running query")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var myInt int
		_ = rows.Scan(&myInt)

		log.Printf("Got: %d", myInt)
	}
}

func getInstanceExample(dm DependencyManager) {
	if conn, ok := dm.GetInstance("connFactory").(*pgxpool.Conn); !ok {
		log.Printf("Failed to get a connection factory")
	} else {
		defer conn.Release()
		rows, err := conn.Query(context.Background(), "SELECT 1;")
		if err != nil {
			log.Printf("Error running query")
			return
		}
		defer rows.Close()

		for rows.Next() {
			var myInt int
			_ = rows.Scan(&myInt)

			log.Printf("Got: %d", myInt)
		}
	}
}
