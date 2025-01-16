package redisson

import (
	"testing"
)

func TestRedissonAtomicLongGetAndSet(t *testing.T) {

	al := GetRedisson().GetAtomicLong("test1")
	at, err := al.GetAndSet(12)
	if err != nil {
		t.Fatal(err)
	}
	if at != 0 {
		panic("at should equal to 0")
	}
}

func TestRedissonAtomicLongGetZero(t *testing.T) {
	al := GetRedisson().GetAtomicLong("test2")
	v, err := al.Get()
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Fatalf("al.Get()=%v", v)
	}
}

func TestRedissonAtomicLongGetAndDelete(t *testing.T) {
	al := GetRedisson().GetAtomicLong("test3")
	err := al.Set(10)
	if err != nil {
		t.Fatal(err)
	}
	v, err := al.Get()
	if v != 10 {
		t.FailNow()
	}
	ad2 := GetRedisson().GetAtomicLong("test4")
	v, err = ad2.GetAndDelete()
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.FailNow()
	}
}

func TestRedissonAtomicLongCompareAndSet(t *testing.T) {
	al := GetRedisson().GetAtomicLong("test5")
	if ok, err := al.CompareAndSet(-1, 2); err != nil {
		t.Fatal(err)
	} else if ok {
		t.FailNow()
	}
	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 0 {
		t.FailNow()
	}

	if ok, err := al.CompareAndSet(0, 2); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.FailNow()
	}
	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 2 {
		t.FailNow()
	}

}

func TestRedissonAtomicLongSetThenIncreament(t *testing.T) {
	al := GetRedisson().GetAtomicLong("test6")
	if err := al.Set(2); err != nil {
		t.Fatal(err)
	}
	if v, err := al.GetAndIncrement(); err != nil {
		t.Fatal(err)
	} else if v != 2 {
		t.FailNow()
	}

	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 3 {
		t.FailNow()
	}
}

func TestRedissonAtomicLongDecrementAndGet(t *testing.T) {
	al := GetRedisson().GetAtomicLong("test7")
	err := al.Set(19)
	if err != nil {
		t.Fatal(err)
	}
	v := al.DecrementAndGet()
	if v != 18 {
		t.Fatalf("v=%v", v)
	}
	v, err = al.Get()
	if err != nil {
		panic(err)
	}
	if v != 18 {
		t.Fatalf("v=%v", v)
	}
}

func TestRedissonAtomicLongIncrementAndGet(t *testing.T) {
	al := GetRedisson().GetAtomicLong("test8")
	if al.IncrementAndGet() != 1 {
		t.FailNow()
	}
	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 1 {
		t.FailNow()
	}
}

func TestRedissonAtomicLongGetAndIncrement(t *testing.T) {
	al := GetRedisson().GetAtomicLong("test9")
	if v, err := al.GetAndIncrement(); err != nil {
		t.Fatal(err)
	} else if v != 0 {
		t.FailNow()
	}

	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 1 {
		t.FailNow()
	}
}

func TestRedissonAtomicLong(t *testing.T) {
	al := GetRedisson().GetAtomicLong("longtest10")
	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 0 {
		t.FailNow()
	}

	if v, err := al.GetAndIncrement(); err != nil {
		t.Fatal(err)
	} else if v != 0 {

	}

	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 1 {
		t.FailNow()
	}

	if v, err := al.GetAndDecrement(); err != nil {
		t.Fatal(err)
	} else if v != 1 {

	}

	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 0 {
		t.FailNow()
	}

	if v, err := al.GetAndIncrement(); err != nil {
		t.Fatal(err)
	} else if v != 0 {

	}

	if v, err := al.GetAndSet(12); err != nil {
		t.Fatal(err)
	} else if v != 1 {
		t.FailNow()
	}

	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 12 {
		t.FailNow()
	}

	if err := al.Set(1); err != nil {
		t.FailNow()
	}
	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 1 {
		t.FailNow()
	}

}
