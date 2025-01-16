package redisson

import "testing"

func TestGetAndSet(t *testing.T) {
	al := GetRedisson().GetAtomicDouble("test1")
	at, err := al.GetAndSet(12)
	if err != nil {
		t.Fatal(err)
	}
	if at != 0 {
		panic("at should equal to 0")
	}
}

func TestGetZero(t *testing.T) {
	al := GetRedisson().GetAtomicDouble("test2")
	v, err := al.Get()
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Fatalf("al.Get()=%v", v)
	}
}

func TestGetAndDelete(t *testing.T) {
	al := GetRedisson().GetAtomicDouble("test3")
	err := al.Set(10.34)
	if err != nil {
		t.Fatal(err)
	}
	v, err := al.Get()
	if v != 10.34 {
		t.FailNow()
	}
	ad2 := GetRedisson().GetAtomicDouble("test4")
	v, err = ad2.GetAndDelete()
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.FailNow()
	}
}

func TestCompareAndSet(t *testing.T) {
	al := GetRedisson().GetAtomicDouble("test5")
	if ok, err := al.CompareAndSet(-1, 2.5); err != nil {
		t.Fatal(err)
	} else if ok {
		t.FailNow()
	}
	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 0 {
		t.FailNow()
	}

	if ok, err := al.CompareAndSet(0, 2.5); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.FailNow()
	}
	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 2.5 {
		t.FailNow()
	}

}

func TestSetThenIncreament(t *testing.T) {
	al := GetRedisson().GetAtomicDouble("test6")
	if err := al.Set(2.81); err != nil {
		t.Fatal(err)
	}
	if v, err := al.GetAndIncrement(); err != nil {
		t.Fatal(err)
	} else if v != 2.81 {
		t.FailNow()
	}

	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 3.81 {
		t.FailNow()
	}
}

func TestDecrementAndGet(t *testing.T) {
	al := GetRedisson().GetAtomicDouble("test7")
	err := al.Set(19.30)
	if err != nil {
		t.Fatal(err)
	}
	v := al.DecrementAndGet()
	if v != 18.30 {
		t.Fatalf("v=%v", v)
	}
	v, err = al.Get()
	if err != nil {
		panic(err)
	}
	if v != 18.30 {
		t.Fatalf("v=%v", v)
	}
}

func TestIncrementAndGet(t *testing.T) {
	al := GetRedisson().GetAtomicDouble("test8")
	if al.IncrementAndGet() != 1 {
		t.FailNow()
	}
	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 1 {
		t.FailNow()
	}
}

func TestGetAndIncrement(t *testing.T) {
	al := GetRedisson().GetAtomicDouble("test9")
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

func Test(t *testing.T) {
	al := GetRedisson().GetAtomicDouble("test10")
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

	if v, err := al.GetAndSet(12.8012); err != nil {
		t.Fatal(err)
	} else if v != 1 {
		t.FailNow()
	}

	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 12.8012 {
		t.FailNow()
	}

	if err := al.Set(1.00123); err != nil {
		t.FailNow()
	}
	if v, err := al.Get(); err != nil {
		t.Fatal(err)
	} else if v != 1.00123 {
		t.FailNow()
	}

}
